package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Port    string
	DataDir string
	WebRoot string
	FFmpeg  string
	FFprobe string
	Library []string
	FFCaps  FFmpegCaps

	// WatchInterval controls automatic library re-scanning when files change.
	// Zero disables the watcher.
	WatchInterval time.Duration

	// Hardware transcoding (optional). HWEnc is "" when unavailable; then
	// transcoding falls back to software (libx264) using FFmpeg above.
	HWFFmpeg string
	HWEnc    string
	HWEnv    []string
	HWDevice string
}

func Load() *Config {
	port := getenv("FLYCAST_PORT", "8100")
	home := getenv("FLYCAST_HOME", filepath.Join(os.Getenv("HOME"), ".flycast"))
	webroot := getenv("FLYCAST_WEBROOT", "./app/web")
	ffprobe := getenv("FFPROBE_PATH", "ffprobe")

	caps := SelectFFmpeg()
	ffmpeg := caps.Cmd
	hw := SelectHardware()

	// Automatic scan-on-change watcher. Enabled by default every 60s; set
	// FLYCAST_WATCH_INTERVAL=0 to disable.
	watchInterval := 60 * time.Second
	if v := os.Getenv("FLYCAST_WATCH_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			watchInterval = time.Duration(n) * time.Second
		}
	}

	lib := []string{}
	if v := os.Getenv("FLYCAST_LIBRARY"); v != "" {
		parts := filepath.SplitList(v)
		if len(parts) == 0 {
			for _, p := range splitComma(v) {
				if p != "" {
					lib = append(lib, p)
				}
			}
		} else {
			lib = append(lib, parts...)
		}
	}

	return &Config{
		Port:          port,
		DataDir:       home,
		WebRoot:       webroot,
		FFmpeg:        ffmpeg,
		FFprobe:       ffprobe,
		Library:       lib,
		FFCaps:        caps,
		WatchInterval: watchInterval,
		HWFFmpeg:      hw.Cmd,
		HWEnc:         hw.Enc,
		HWEnv:         hw.Env,
		HWDevice:      hw.Device,
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitComma(s string) []string {
	// simple
	res := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				res = append(res, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		res = append(res, cur)
	}
	return res
}
