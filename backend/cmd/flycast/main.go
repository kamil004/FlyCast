package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/kamil/flycast/internal/api"
	"github.com/kamil/flycast/internal/auth"
	"github.com/kamil/flycast/internal/config"
	"github.com/kamil/flycast/internal/db"
	"github.com/kamil/flycast/internal/scanner"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	auth.InitJWTSecret()
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "thumbs"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "db"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "logs"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "cache"), 0755); err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(filepath.Join(cfg.DataDir, "db", "flycast.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	app := fiber.New(fiber.Config{
		BodyLimit: 1024 * 1024 * 1024,
	})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	api.Setup(app, database, cfg)
	go scanner.Start(database, cfg)

	log.Printf("flycast: ffmpeg=%q h264enc=%v h264dec=%v aacenc=%v",
		cfg.FFmpeg, cfg.FFCaps.H264Enc, cfg.FFCaps.H264Dec, cfg.FFCaps.AACEnc)
	if cfg.HWEnc != "" {
		log.Printf("flycast: hardware transcode=%s ffmpeg=%q device=%s env=%v",
			cfg.HWEnc, cfg.HWFFmpeg, cfg.HWDevice, cfg.HWEnv)
	} else {
		log.Printf("flycast: hardware transcode unavailable; using software libx264")
	}
	log.Printf("FlyCast listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
