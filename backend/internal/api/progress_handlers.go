package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

func saveProgress(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		vid := c.Params("id")
		var body struct {
			Position float64 `json:"position"`
			Duration float64 `json:"duration"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		_, _ = db.Exec(`
			INSERT INTO user_progress(user_id, video_id, position, duration, last_updated)
			VALUES(?,?,?,?,CURRENT_TIMESTAMP)
			ON CONFLICT(user_id, video_id) DO UPDATE SET
				position=excluded.position,
				duration=excluded.duration,
				last_updated=CURRENT_TIMESTAMP
		`, uid, vid, body.Position, body.Duration)
		return c.JSON(fiber.Map{"ok": true})
	}
}

func getProgress(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		vid := c.Params("id")
		var pos, dur float64
		var last sql.NullString
		_ = db.QueryRow("SELECT position, duration, last_updated FROM user_progress WHERE user_id=? AND video_id=?", uid, vid).Scan(&pos, &dur, &last)
		return c.JSON(fiber.Map{
			"position":     pos,
			"duration":     dur,
			"last_updated": last.String,
		})
	}
}
