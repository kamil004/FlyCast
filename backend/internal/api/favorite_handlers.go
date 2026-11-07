package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

// listFavorites returns the current user's favourite videos.
func listFavorites(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		sql := `
			SELECT v.id, v.filename, v.title, v.year, v.duration, v.size, v.thumb_path, v.created_at, v.mtime
			FROM favorites f
			JOIN videos v ON v.id = f.video_id
			WHERE f.user_id=?`
		args := []any{uid}
		if expr, fargs := pathFilter(db, uid, "v.path"); expr != "" {
			sql += " AND " + expr
			args = append(args, fargs...)
		}
		sql += " ORDER BY f.created_at DESC"
		rows, err := db.Query(sql, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		return c.JSON(fiber.Map{"items": scanVideoItems(rows)})
	}
}

// favoriteIDs returns just the ids of the user's favourites (for star state).
func favoriteIDs(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		rows, err := db.Query("SELECT video_id FROM favorites WHERE user_id=?", uid)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		return c.JSON(fiber.Map{"ids": ids})
	}
}

func addFavorite(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		id := c.Params("id")
		if id == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_id"})
		}
		if _, err := db.Exec("INSERT OR IGNORE INTO favorites(user_id, video_id) VALUES(?,?)", uid, id); err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true, "favorite": true})
	}
}

func removeFavorite(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		id := c.Params("id")
		_, _ = db.Exec("DELETE FROM favorites WHERE user_id=? AND video_id=?", uid, id)
		return c.JSON(fiber.Map{"ok": true, "favorite": false})
	}
}
