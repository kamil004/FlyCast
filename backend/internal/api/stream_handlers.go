package api

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/kamil/flycast/internal/config"
	"github.com/kamil/flycast/internal/scanner"
)

func streamHandler(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var path string
		if err := db.QueryRow("SELECT path FROM videos WHERE id=?", id).Scan(&path); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		if uid, _ := c.Locals("uid").(string); !canAccessPath(db, uid, path) {
			return c.Status(403).JSON(fiber.Map{"err": "forbidden"})
		}
		if _, err := os.Stat(path); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "file_missing"})
		}

		var vcodec, acodec sql.NullString
		_ = db.QueryRow("SELECT vcodec, acodec FROM videos WHERE id=?", id).Scan(&vcodec, &acodec)
		if !vcodec.Valid || vcodec.String == "" {
			// lazy probe for libraries scanned before codec support was added
			mi := scanner.Probe(cfg.FFmpeg, path)
			if mi.VCodec != "" || mi.ACodec != "" || mi.Duration > 0 {
				_, _ = db.Exec("UPDATE videos SET vcodec=?, acodec=?, duration=?, width=?, height=? WHERE id=?",
					mi.VCodec, mi.ACodec, mi.Duration, mi.Width, mi.Height, id)
			}
			vcodec.String, vcodec.Valid = mi.VCodec, true
			acodec.String, acodec.Valid = mi.ACodec, true
		}

		ext := strings.ToLower(filepath.Ext(path))
		canEncode := cfg.FFCaps.H264Enc
		bitrate := 0 // 0 = source quality
		if v := c.Query("bitrate"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				bitrate = n
			}
		}
		// q parameter: quality percentage (20-100); converts to bitrate
		if v := c.Query("q"); v != "" && bitrate == 0 {
			if p, err := strconv.Atoi(v); err == nil && p >= 20 && p <= 100 {
				bitrate = p * 80 // 100% = 8000kbps, 50% = 4000kbps, etc.
			}
		}
		start := 0.0 // 0 = from the beginning; >0 seeks before transcoding
		if v := c.Query("start"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				start = f
			}
		}
		switch c.Query("transcode", "") {
		case "1":
			return transcodeStream(c, cfg, path, bitrate, start)
		case "0":
			return c.SendFile(path)
		}
		// An explicit target bitrate always means "re-encode to it".
		if bitrate > 0 {
			return transcodeStream(c, cfg, path, bitrate, start)
		}
		plan := planStream(ext, vcodec.String, acodec.String, canEncode)
		switch plan.Mode {
		case modeDirect:
			return c.SendFile(path)
		case modeRemux:
			return remuxStream(c, cfg, path, plan.Mime, start)
		case modeTranscode:
			return transcodeStream(c, cfg, path, 0, start)
		default:
			return c.Status(415).JSON(fiber.Map{"err": "unsupported_format"})
		}
	}
}

type streamMode int

const (
	modeDirect streamMode = iota
	modeRemux
	modeTranscode
	modeUnsupported
)

func (m streamMode) String() string {
	switch m {
	case modeDirect:
		return "direct"
	case modeRemux:
		return "remux"
	case modeTranscode:
		return "transcode"
	default:
		return "unsupported"
	}
}

type streamPlan struct {
	Mode streamMode
	Mime string
}

// planStream decides how to deliver a file to a browser: play it directly,
// remux (copy streams into a different container) or fully transcode.
func planStream(ext, vcodec, acodec string, canEncode bool) streamPlan {
	if canDirectPlay(ext, vcodec, acodec) {
		return streamPlan{Mode: modeDirect, Mime: guessMime(ext)}
	}
	if mime, ok := remuxTarget(vcodec, acodec); ok {
		return streamPlan{Mode: modeRemux, Mime: mime}
	}
	if canEncode {
		return streamPlan{Mode: modeTranscode, Mime: "video/mp4"}
	}
	return streamPlan{Mode: modeUnsupported, Mime: "video/mp4"}
}

// remuxTarget returns the container MIME type we can remux (stream-copy) into
// when both the video and audio codecs are browser-compatible.
func remuxTarget(vcodec, acodec string) (string, bool) {
	videoIn := func(ok ...string) bool {
		for _, k := range ok {
			if vcodec == k {
				return true
			}
		}
		return false
	}
	audioIn := func(ok ...string) bool {
		if acodec == "" {
			return false
		}
		for _, k := range ok {
			if acodec == k {
				return true
			}
		}
		return false
	}
	switch {
	case videoIn("vp8", "vp9", "av1"):
		if audioIn("opus", "vorbis") {
			return "video/webm", true
		}
	case videoIn("h264"):
		if audioIn("aac", "mp3", "opus") {
			return "video/mp4", true
		}
	}
	return "", false
}

// canDirectPlay reports whether the browser can play the file as-is.
func canDirectPlay(ext, vcodec, acodec string) bool {
	videoOK := func(ok ...string) bool {
		if vcodec == "" {
			return true
		}
		for _, k := range ok {
			if vcodec == k {
				return true
			}
		}
		return false
	}
	audioOK := func(ok ...string) bool {
		if acodec == "" {
			return true
		}
		for _, k := range ok {
			if acodec == k {
				return true
			}
		}
		return false
	}
	switch ext {
	case ".webm":
		return videoOK("vp8", "vp9", "av1") && audioOK("opus", "vorbis")
	case ".mp4", ".m4v":
		return videoOK("h264", "vp9", "av1") && audioOK("aac", "mp3", "opus")
	case ".mov":
		return videoOK("h264") && audioOK("aac", "mp3")
	default:
		return false
	}
}

// ffmpegRun describes a single ffmpeg invocation to try.
type ffmpegRun struct {
	name string
	env  []string
	args []string
}

// pipeFFmpeg runs ffmpeg with the given args and streams its stdout to the
// client. extraEnv is appended to the process environment (used for the LIBVA_*
// variables the hardware encoder needs). The process is reaped in the
// background; on client disconnect ffmpeg receives EPIPE on its next write and
// exits on its own.
func pipeFFmpeg(c *fiber.Ctx, ffmpeg string, extraEnv []string, args []string) error {
	return streamFFmpeg(c, []ffmpegRun{{name: ffmpeg, env: extraEnv, args: args}})
}

// streamFFmpeg tries each run in order and streams the first one that produces
// output. A run is considered failed when ffmpeg exits before emitting a single
// byte (e.g. the Quick Sync encoder rejecting a 10-bit source); in that case the
// next run (typically a software encoder) is attempted, so a file that the
// hardware cannot handle still plays instead of showing an empty error stream.
func streamFFmpeg(c *fiber.Ctx, runs []ffmpegRun) error {
	var lastErr string
	for _, r := range runs {
		cmd := exec.Command(r.name, r.args...)
		if len(r.env) > 0 {
			cmd.Env = append(os.Environ(), r.env...)
		}
		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			lastErr = err.Error()
			continue
		}
		if err := cmd.Start(); err != nil {
			lastErr = err.Error()
			continue
		}
		head := make([]byte, 1)
		n, _ := io.ReadFull(stdout, head)
		if n == 0 {
			// ffmpeg failed before producing anything: reap it and try the next.
			_ = cmd.Wait()
			lastErr = strings.TrimSpace(errBuf.String())
			continue
		}
		go func() { _ = cmd.Wait() }()
		return c.SendStream(io.MultiReader(bytes.NewReader(head[:n]), stdout))
	}
	if lastErr == "" {
		lastErr = "ffmpeg_error"
	}
	return c.Status(500).SendString("ffmpeg_error: " + lastErr)
}

func remuxStream(c *fiber.Ctx, cfg *config.Config, path, mime string, start float64) error {
	c.Set("Content-Type", mime)
	args := []string{"-hide_banner", "-loglevel", "error"}
	args = append(args, seekArgs(start)...)
	if mime == "video/webm" {
		args = append(args, "-i", path,
			"-c", "copy", "-f", "webm", "pipe:1")
	} else {
		args = append(args, "-i", path,
			"-c", "copy", "-movflags", "frag_keyframe+empty_moov+default_base_moof",
			"-f", "mp4", "pipe:1")
	}
	return pipeFFmpeg(c, cfg.FFmpeg, nil, args)
}

// seekArgs returns the input-seek options (placed before -i) for the given
// start offset in seconds. An input seek is fast (no decode of skipped frames)
// and, for stream-copy remuxes, snaps to the nearest keyframe.
func seekArgs(start float64) []string {
	if start <= 0 {
		return nil
	}
	return []string{"-ss", strconv.FormatFloat(start, 'f', 3, 64)}
}

// softwareTranscodeArgs builds the libx264 argument list used when no hardware
// encoder is available, or as a fallback when the hardware encoder fails.
func softwareTranscodeArgs(path string, bitrate int, start float64) []string {
	abits := "160k"
	if bitrate > 0 && bitrate <= 800 {
		abits = "96k"
	}

	var vargs []string
	if bitrate > 0 {
		// Keep the source resolution (only ensure even dimensions), but cap the
		// bitrate so the transfer adapts to the connection.
		vargs = []string{"-c:v", "libx264", "-preset", "veryfast",
			"-b:v", fmt.Sprintf("%dk", bitrate),
			"-maxrate", fmt.Sprintf("%dk", bitrate*3/2),
			"-bufsize", fmt.Sprintf("%dk", bitrate*2),
			"-profile:v", "main", "-level", "4.1",
			"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p",
		}
	} else {
		vargs = []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
			"-profile:v", "high", "-level", "4.1",
			"-vf", "scale='min(1920,iw)':-2,format=yuv420p",
		}
	}

	args := []string{"-hide_banner", "-loglevel", "error"}
	args = append(args, seekArgs(start)...)
	args = append(args, "-i", path)
	args = append(args, vargs...)
	args = append(args,
		"-c:a", "aac", "-b:a", abits, "-ac", "2",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4", "pipe:1",
	)
	return args
}

// transcodeStream re-encodes to H.264/AAC in a fragmented MP4 so the browser can
// start playing immediately. The resolution is kept; bitrate > 0 caps the video
// bitrate (kbps) to throttle bandwidth on slow links. When a hardware encoder
// (Intel Quick Sync / VA-API) is available, it is used so the weak CPU is not
// saturated; otherwise it falls back to software libx264.
func transcodeStream(c *fiber.Ctx, cfg *config.Config, path string, bitrate int, start float64) error {
	c.Set("Content-Type", "video/mp4")
	if cfg.HWEnc != "" && cfg.HWFFmpeg != "" {
		return hwTranscodeStream(c, cfg, path, bitrate, start)
	}
	return pipeFFmpeg(c, cfg.FFmpeg, nil, softwareTranscodeArgs(path, bitrate, start))
}

// hwTranscodeArgs builds the hardware-encoder argument list for the configured
// encoder. ok is false when no supported hardware encoder is configured.
func hwTranscodeArgs(cfg *config.Config, path string, bitrate int, start float64) ([]string, bool) {
	abits := "160k"
	if bitrate <= 800 {
		abits = "96k"
	}
	rate := []string{
		"-b:v", fmt.Sprintf("%dk", bitrate),
		"-maxrate", fmt.Sprintf("%dk", bitrate*3/2),
		"-bufsize", fmt.Sprintf("%dk", bitrate*2),
	}

	args := []string{"-hide_banner", "-loglevel", "error"}
	switch cfg.HWEnc {
	case "qsv":
		// -hwaccel qsv decodes on the GPU; -pix_fmt nv12 converts the 10-bit
		// (p010) frames that the QSV encoder cannot accept on its own, which is
		// what made 10-bit HEVC (HDR) files fail with "parameters not
		// supported by the QSV runtime".
		args = append(args, "-hwaccel", "qsv")
		args = append(args, seekArgs(start)...)
		args = append(args, "-i", path, "-c:v", "h264_qsv", "-preset", "veryfast", "-pix_fmt", "nv12")
		args = append(args, rate...)
	case "vaapi":
		dev := cfg.HWDevice
		if dev == "" {
			dev = "/dev/dri/renderD128"
		}
		args = append(args, "-vaapi_device", dev)
		args = append(args, seekArgs(start)...)
		args = append(args, "-i", path,
			"-vf", "format=nv12,hwupload", "-c:v", "h264_vaapi")
		args = append(args, rate...)
	default:
		return nil, false
	}

	args = append(args,
		"-c:a", "aac", "-b:a", abits, "-ac", "2",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4", "pipe:1",
	)
	return args, true
}

// hwTranscodeStream re-encodes with a hardware encoder and transparently falls
// back to software libx264 when the hardware path fails to produce any output.
func hwTranscodeStream(c *fiber.Ctx, cfg *config.Config, path string, bitrate int, start float64) error {
	if bitrate <= 0 {
		bitrate = 4000
	}
	hwArgs, ok := hwTranscodeArgs(cfg, path, bitrate, start)
	if !ok {
		return transcodeStream(c, cfg, path, bitrate, start)
	}
	runs := []ffmpegRun{
		{name: cfg.HWFFmpeg, env: cfg.HWEnv, args: hwArgs},
		{name: cfg.FFmpeg, args: softwareTranscodeArgs(path, bitrate, start)},
	}
	return streamFFmpeg(c, runs)
}

// pickThumbTime chooses a safe timestamp (10% of the clip, capped at 5s) for
// frame extraction. Short clips would otherwise yield no frame at a fixed 5s.
func pickThumbTime(duration float64) string {
	if duration <= 0 {
		return "00:00:01"
	}
	ts := duration * 0.1
	if ts > 5 {
		ts = 5
	}
	return fmt.Sprintf("%.2f", ts)
}

func thumbHandler(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var vpath string
		_ = db.QueryRow("SELECT path FROM videos WHERE id=?", id).Scan(&vpath)
		if uid, _ := c.Locals("uid").(string); !canAccessPath(db, uid, vpath) {
			return c.Status(403).JSON(fiber.Map{"err": "forbidden"})
		}
		var thumb string
		_ = db.QueryRow("SELECT thumb_path FROM videos WHERE id=?", id).Scan(&thumb)
		if thumb != "" && exists(thumb) {
			return c.SendFile(thumb)
		}
		var path string
		var duration sql.NullFloat64
		if err := db.QueryRow("SELECT path, duration FROM videos WHERE id=?", id).Scan(&path, &duration); err != nil || path == "" {
			return c.SendStatus(404)
		}
		out := filepath.Join(cfg.DataDir, "thumbs", id+".jpg")
		ss := pickThumbTime(duration.Float64)
		_ = exec.Command(cfg.FFmpeg, "-y", "-ss", ss, "-i", path, "-vframes", "1", "-vf", "scale=480:-2", "-q:v", "4", out).Run()
		if exists(out) {
			_, _ = db.Exec("UPDATE videos SET thumb_path=? WHERE id=?", out, id)
			return c.SendFile(out)
		}
		return c.SendStatus(404)
	}
}

func guessMime(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".m4v":
		return "video/mp4"
	case ".ts":
		return "video/mp2t"
	case ".mpg", ".mpeg":
		return "video/mpeg"
	default:
		return "application/octet-stream"
	}
}
