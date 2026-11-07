package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func listAlbums(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		rows, err := db.Query(`
			SELECT a.id, a.name, a.cover_video_id, a.created_at,
			       (SELECT COUNT(*) FROM album_items ai WHERE ai.album_id = a.id) AS cnt
			FROM albums a WHERE a.user_id=? ORDER BY a.created_at DESC`, uid)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		albums := []fiber.Map{}
		for rows.Next() {
			var id, name, cover, created sql.NullString
			var cnt int
			_ = rows.Scan(&id, &name, &cover, &created, &cnt)
			albums = append(albums, fiber.Map{
				"id":             id.String,
				"name":           name.String,
				"cover_video_id": cover.String,
				"created_at":     created.String,
				"count":          cnt,
			})
		}
		return c.JSON(fiber.Map{"albums": albums})
	}
}

func createAlbum(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		var body struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		if body.Name == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_name"})
		}
		id := uuid.NewString()
		_, err := db.Exec("INSERT INTO albums(id, name, user_id) VALUES(?,?,?)", id, body.Name, uid)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		return c.JSON(fiber.Map{"id": id, "name": body.Name})
	}
}

func addToAlbum(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		albumID := c.Params("id")
		var body struct {
			VideoID string `json:"video_id"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		if body.VideoID == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_video_id"})
		}
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM album_items WHERE album_id=? AND video_id=?", albumID, body.VideoID).Scan(&cnt)
		if cnt == 0 {
			_, _ = db.Exec("INSERT OR IGNORE INTO album_items(album_id, video_id, sort_order) VALUES(?,?, (SELECT IFNULL(MAX(sort_order)+1,0) FROM album_items WHERE album_id=?))", albumID, body.VideoID, albumID)
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func deleteAlbum(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		id := c.Params("id")
		var owner sql.NullString
		if err := db.QueryRow("SELECT user_id FROM albums WHERE id=?", id).Scan(&owner); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		if owner.String != uid {
			return c.Status(403).JSON(fiber.Map{"err": "forbidden"})
		}
		_, _ = db.Exec("DELETE FROM album_items WHERE album_id=?", id)
		if _, err := db.Exec("DELETE FROM albums WHERE id=?", id); err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// removeFromAlbum removes a single video from an album.
func removeFromAlbum(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		id := c.Params("id")
		var owner sql.NullString
		if err := db.QueryRow("SELECT user_id FROM albums WHERE id=?", id).Scan(&owner); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		if owner.String != uid {
			return c.Status(403).JSON(fiber.Map{"err": "forbidden"})
		}
		videoID := c.Query("video_id", "")
		if videoID == "" {
			var body struct {
				VideoID string `json:"video_id"`
			}
			_ = c.BodyParser(&body)
			videoID = body.VideoID
		}
		if videoID == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_video_id"})
		}
		_, _ = db.Exec("DELETE FROM album_items WHERE album_id=? AND video_id=?", id, videoID)
		return c.JSON(fiber.Map{"ok": true})
	}
}

func getAlbum(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		albumID := c.Params("id")
		var name, cover sql.NullString
		_ = db.QueryRow("SELECT name, cover_video_id FROM albums WHERE id=?", albumID).Scan(&name, &cover)
		rows, err := db.Query(`
			SELECT v.id, v.path, v.filename, v.title, v.thumb_path, v.duration, v.size
			FROM album_items ai
			JOIN videos v ON v.id = ai.video_id
			WHERE ai.album_id=?
			ORDER BY ai.sort_order ASC, v.filename ASC
		`, albumID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		videos := []fiber.Map{}
		for rows.Next() {
			var id, path, filename, title, thumb sql.NullString
			var dur, size sql.NullFloat64
			_ = rows.Scan(&id, &path, &filename, &title, &thumb, &dur, &size)
			display := title.String
			if display == "" {
				display = filename.String
			}
			videos = append(videos, fiber.Map{
				"type":     "video",
				"id":       id.String,
				"path":     path.String,
				"filename": filename.String,
				"title":    display,
				"thumb":    thumbURL(id.String, thumb),
				"duration": dur.Float64,
				"size":     int64(size.Float64),
			})
		}
		return c.JSON(fiber.Map{
			"id":             albumID,
			"name":           name.String,
			"cover_video_id": cover.String,
			"items":          videos,
		})
	}
}
