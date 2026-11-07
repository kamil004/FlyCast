package api

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/kamil/flycast/internal/config"
	"github.com/kamil/flycast/internal/scanner"
)

// convertJob tracks the single in-flight "convert to MP4" re-encode. Only one
// conversion runs at a time because it saturates the already weak CPU.
type convertJob struct {
	mu       sync.Mutex
	running  bool
	cancel   bool
	videoID  string
	src      string
	dst      string
	duration float64
	percent  float64
	err      string
}

var convJob convertJob

// targetBitrateKbps picks a video bitrate for the re-encode from the source
// size/duration (kept close to the source so quality is preserved), clamped to
// sane bounds, with a resolution-based fallback when the source is unknown.
func targetBitrateKbps(height int, sizeBytes, dur float64) int {
	src := 0
	if sizeBytes > 0 && dur > 0 {
		src = int((sizeBytes * 8) / dur / 1000)
	}
	if src <= 0 {
		switch {
		case height >= 1800:
			return 12000
		case height >= 1000:
			return 6000
		case height >= 700:
			return 3500
		default:
			return 1500
		}
	}
	if src < 2500 {
		src = 2500
	}
	if src > 18000 {
		src = 18000
	}
	return src
}

// mp4OutputPath returns a sibling .mp4 path for src, never clobbering an
// existing file (the source itself may already be an .mp4).
func mp4OutputPath(src string) string {
	dir := filepath.Dir(src)
	stem := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	cand := filepath.Join(dir, stem+".mp4")
	if !strings.EqualFold(cand, src) && !exists(cand) {
		return cand
	}
	for i := 2; ; i++ {
		cand = filepath.Join(dir, fmt.Sprintf("%s (%d).mp4", stem, i))
		if !exists(cand) {
			return cand
		}
	}
}

// convertSWArgs builds a software libx264 re-encode of src into dst. Progress is
// written to stdout as key=value lines (see runOneConvert).
func convertSWArgs(src, dst string, kbps int) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-nostats", "-progress", "pipe:1", "-y",
		"-i", src,
		"-c:v", "libx264", "-preset", "veryfast",
		"-b:v", fmt.Sprintf("%dk", kbps),
		"-maxrate", fmt.Sprintf("%dk", kbps*3/2),
		"-bufsize", fmt.Sprintf("%dk", kbps*2),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k",
		"-movflags", "+faststart",
		"-f", "mp4", dst,
	}
}

// convertHWArgs builds a hardware re-encode of src into dst. ok is false when no
// supported hardware encoder is configured.
func convertHWArgs(cfg *config.Config, src, dst string, kbps int) ([]string, bool) {
	rate := []string{
		"-b:v", fmt.Sprintf("%dk", kbps),
		"-maxrate", fmt.Sprintf("%dk", kbps*3/2),
		"-bufsize", fmt.Sprintf("%dk", kbps*2),
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-nostats", "-progress", "pipe:1", "-y"}
	switch cfg.HWEnc {
	case "qsv":
		// -hwaccel qsv decodes on the GPU; -pix_fmt nv12 converts the 10-bit
		// (p010) frames that the QSV encoder cannot accept on its own.
		args = append(args, "-hwaccel", "qsv", "-i", src,
			"-c:v", "h264_qsv", "-preset", "veryfast", "-pix_fmt", "nv12")
		args = append(args, rate...)
	case "vaapi":
		dev := cfg.HWDevice
		if dev == "" {
			dev = "/dev/dri/renderD128"
		}
		args = append(args, "-vaapi_device", dev, "-i", src,
			"-vf", "format=nv12,hwupload", "-c:v", "h264_vaapi")
		args = append(args, rate...)
	default:
		return nil, false
	}
	args = append(args,
		"-c:a", "aac", "-b:a", "192k",
		"-movflags", "+faststart",
		"-f", "mp4", dst,
	)
	return args, true
}

// buildConvertRuns returns the encoder attempts in order: the hardware encoder
// (fast) first, then software libx264 as a fallback.
func buildConvertRuns(cfg *config.Config, src, dst string, kbps int) []ffmpegRun {
	var runs []ffmpegRun
	if args, ok := convertHWArgs(cfg, src, dst, kbps); ok {
		runs = append(runs, ffmpegRun{name: cfg.HWFFmpeg, env: cfg.HWEnv, args: args})
	}
	runs = append(runs, ffmpegRun{name: cfg.FFmpeg, args: convertSWArgs(src, dst, kbps)})
	return runs
}

// convertVideoHandler starts the background re-encode. Admin-only (wired into
// the admin route group).
func convertVideoHandler(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var path string
		var dur, size sql.NullFloat64
		var height sql.NullInt64
		if err := db.QueryRow("SELECT path, duration, size, height FROM videos WHERE id=?", id).Scan(&path, &dur, &size, &height); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		if _, err := os.Stat(path); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "file_missing"})
		}
		dst := mp4OutputPath(path)
		kbps := targetBitrateKbps(int(height.Int64), size.Float64, dur.Float64)

		convJob.mu.Lock()
		if convJob.running {
			convJob.mu.Unlock()
			return c.Status(409).JSON(fiber.Map{"err": "busy", "video_id": convJob.videoID})
		}
		convJob.running = true
		convJob.cancel = false
		convJob.videoID = id
		convJob.src = path
		convJob.dst = dst
		convJob.duration = dur.Float64
		convJob.percent = 0
		convJob.err = ""
		convJob.mu.Unlock()

		go runConversion(cfg, path, dst, dur.Float64, kbps)
		return c.JSON(fiber.Map{"ok": true, "dst": dst, "bitrate": kbps})
	}
}

func runConversion(cfg *config.Config, src, dst string, duration float64, kbps int) {
	runs := buildConvertRuns(cfg, src, dst, kbps)
	var lastErr string
	for _, r := range runs {
		err := runOneConvert(r, dst, duration)
		if err == nil {
			convJob.mu.Lock()
			convJob.running = false
			convJob.percent = 100
			convJob.mu.Unlock()
			scanner.Trigger()
			return
		}
		_ = os.Remove(dst)
		lastErr = err.Error()
		convJob.mu.Lock()
		cancelled := convJob.cancel
		convJob.mu.Unlock()
		if cancelled {
			break
		}
	}
	convJob.mu.Lock()
	convJob.running = false
	convJob.err = strings.TrimSpace(lastErr)
	convJob.mu.Unlock()
}

// runOneConvert executes a single ffmpeg run, updating the job progress from
// ffmpeg's -progress output and honouring cancellation.
func runOneConvert(r ffmpegRun, dst string, duration float64) error {
	cmd := exec.Command(r.name, r.args...)
	if len(r.env) > 0 {
		cmd.Env = append(os.Environ(), r.env...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		v, ok := strings.CutPrefix(line, "out_time_us=")
		if !ok {
			continue
		}
		us, e := strconv.ParseInt(v, 10, 64)
		if e != nil || duration <= 0 || us < 0 {
			continue
		}
		pct := float64(us) / 1e6 / duration * 100
		if pct > 100 {
			pct = 100
		}
		convJob.mu.Lock()
		convJob.percent = pct
		cancel := convJob.cancel
		convJob.mu.Unlock()
		if cancel {
			_ = cmd.Process.Kill()
		}
	}
	werr := cmd.Wait()
	convJob.mu.Lock()
	cancelled := convJob.cancel
	convJob.mu.Unlock()
	if cancelled {
		return fmt.Errorf("cancelled")
	}
	if werr != nil {
		if msg := strings.TrimSpace(errBuf.String()); msg != "" {
			return fmt.Errorf("%s: %s", werr, msg)
		}
		return werr
	}
	if !exists(dst) {
		return fmt.Errorf("no output produced")
	}
	return nil
}

// convertJobStatus reports progress of the current/last conversion.
func convertJobStatus(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		convJob.mu.Lock()
		defer convJob.mu.Unlock()
		return c.JSON(fiber.Map{
			"running":  convJob.running,
			"video_id": convJob.videoID,
			"percent":  convJob.percent,
			"error":    convJob.err,
			"dst":      convJob.dst,
		})
	}
}

// convertJobCancel requests cancellation of the running conversion.
func convertJobCancel(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		convJob.mu.Lock()
		if convJob.running {
			convJob.cancel = true
		}
		convJob.mu.Unlock()
		return c.JSON(fiber.Map{"ok": true})
	}
}
