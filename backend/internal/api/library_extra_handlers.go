package api

import (
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/kamil/flycast/internal/config"
)

// thumbURL returns the client-facing thumbnail URL for a video id when a
// thumbnail exists, otherwise it returns an empty string so the client can
// fall back to a placeholder.
func thumbURL(id string, thumb sql.NullString) string {
	if thumb.Valid && thumb.String != "" {
		return "/api/thumb/" + id
	}
	return ""
}

// librariesHandler lists top-level libraries (root folders that contain media).
func librariesHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(folderCountSQL + ` WHERE f.parent_id IS NULL ORDER BY f.name`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		type libRow struct {
			id, path, name string
			count          int
			size           int64
		}
		libs := []libRow{}
		for rows.Next() {
			var id, path, name, thumb sql.NullString
			var cnt int
			var sz sql.NullFloat64
			_ = rows.Scan(&id, &path, &name, &thumb, &cnt, &sz)
			if isHidden(name.String) {
				continue
			}
			if path.Valid && exists(path.String) {
				libs = append(libs, libRow{id.String, path.String, name.String, cnt, int64(sz.Float64)})
			}
		}
		rows.Close()
		uid, _ := c.Locals("uid").(string)
		unrestricted, roots := libAccess(db, uid)
		items := []fiber.Map{}
		for _, l := range libs {
			if !unrestricted && !pathIn(l.path, roots) {
				continue
			}
			_, mediaType := libraryOf(db, l.id)
			items = append(items, fiber.Map{
				"id":         l.id,
				"path":       l.path,
				"name":       l.name,
				"count":      l.count,
				"size":       l.size,
				"media_type": mediaType,
				"thumbs":     libraryThumbs(db, l.id, 4),
			})
		}
		return c.JSON(fiber.Map{"items": items})
	}
}

// folderSubtreeSize returns the total size (bytes) of all videos in a folder's
// entire subtree (any depth).
func folderSubtreeSize(db *sql.DB, folderID string) int64 {
	const q = `
WITH RECURSIVE desc(id) AS (
  SELECT id FROM folders WHERE id = ?
  UNION ALL
  SELECT f.id FROM folders f JOIN desc d ON f.parent_id = d.id
)
SELECT IFNULL(SUM(v.size),0) FROM videos v
WHERE v.folder_id IN (SELECT id FROM desc) AND v.path NOT LIKE '%/.%'`
	var total sql.NullFloat64
	if err := db.QueryRow(q, folderID).Scan(&total); err != nil {
		return 0
	}
	return int64(total.Float64)
}

// libraryThumbs returns up to n thumbnail URLs of the most recently added
// videos anywhere in the library's folder subtree. Used to render a collage
// tile on the home page.
func libraryThumbs(db *sql.DB, folderID string, n int) []string {
	const q = `
WITH RECURSIVE desc(id) AS (
  SELECT id FROM folders WHERE id = ?
  UNION ALL
  SELECT f.id FROM folders f JOIN desc d ON f.parent_id = d.id
)
SELECT v.id FROM videos v
WHERE v.folder_id IN (SELECT id FROM desc)
  AND v.thumb_path IS NOT NULL AND v.thumb_path != ''
  AND v.path NOT LIKE '%/.%'
ORDER BY v.created_at DESC
LIMIT ?`
	rows, err := db.Query(q, folderID, n)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		out = append(out, "/api/thumb/"+id)
	}
	return out
}

// recentHandler returns the most recently added videos.
func recentHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		limit := intParam(c, "limit", 24)
		q := `
			SELECT id, filename, title, year, duration, size, thumb_path, created_at, mtime
			FROM videos
			WHERE path NOT LIKE '%/.%'`
		args := []any{}
		if expr, fargs := pathFilter(db, uid, "path"); expr != "" {
			q += " AND " + expr
			args = append(args, fargs...)
		}
		q += " ORDER BY created_at DESC, filename ASC LIMIT ?"
		args = append(args, limit)
		rows, err := db.Query(q, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		return c.JSON(fiber.Map{"items": scanVideoItems(rows)})
	}
}

// continueHandler returns videos the current user has partially watched.
func continueHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		limit := intParam(c, "limit", 24)
		q := `
			SELECT v.id, v.filename, v.title, v.year, v.duration, v.size, v.thumb_path, v.created_at,
			       up.position, up.duration, v.mtime
			FROM user_progress up
			JOIN videos v ON v.id = up.video_id
			WHERE up.user_id = ?
			  AND up.position > 5
			  AND (up.duration = 0 OR up.position < up.duration - 15)
			  AND v.path NOT LIKE '%/.%'`
		args := []any{uid}
		if expr, fargs := pathFilter(db, uid, "v.path"); expr != "" {
			q += " AND " + expr
			args = append(args, fargs...)
		}
		q += " ORDER BY up.last_updated DESC LIMIT ?"
		args = append(args, limit)
		rows, err := db.Query(q, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		items := []fiber.Map{}
		for rows.Next() {
			var id, filename, title, thumb, created sql.NullString
			var year, mtime sql.NullInt64
			var dur, size, pos, pDur sql.NullFloat64
			_ = rows.Scan(&id, &filename, &title, &year, &dur, &size, &thumb, &created, &pos, &pDur, &mtime)
			m := videoMap(id, filename, title, year, dur, size, thumb, created, mtime)
			m["position"] = pos.Float64
			if pDur.Float64 > 0 {
				m["progress"] = pos.Float64 / pDur.Float64
			} else if dur.Float64 > 0 {
				m["progress"] = pos.Float64 / dur.Float64
			}
			items = append(items, m)
		}
		return c.JSON(fiber.Map{"items": items})
	}
}

// allVideosHandler returns all scanned videos (optionally filtered).
func allVideosHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		folder := strings.TrimSpace(c.Query("folder", ""))
		limit := intParam(c, "limit", 500)

		q := `SELECT id, filename, title, year, duration, size, thumb_path, created_at, mtime
		      FROM videos WHERE path NOT LIKE '%/.%'`
		args := []any{}
		if folder != "" {
			q += " AND folder_id = ?"
			args = append(args, folder)
		}
		if expr, fargs := pathFilter(db, uid, "path"); expr != "" {
			q += " AND " + expr
			args = append(args, fargs...)
		}
		q += " ORDER BY filename ASC LIMIT ?"
		args = append(args, limit)

		rows, err := db.Query(q, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		return c.JSON(fiber.Map{"items": scanVideoItems(rows)})
	}
}

// videoDetailHandler returns full details for a single video.
func videoDetailHandler(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var vid, filename, title, path, thumb, backdrop, overview, imdb, tmdb, source, container sql.NullString
		var vcodec, acodec, created sql.NullString
		var year, mtime sql.NullInt64
		var dur, size, width, height sql.NullFloat64
		err := db.QueryRow(`
			SELECT id, filename, title, path, year, duration, size, thumb_path,
			       backdrop_path, overview, imdb_id, tmdb_id, metadata_source, container,
			       vcodec, acodec, created_at, width, height, mtime
			FROM videos WHERE id = ?
		`, id).Scan(&vid, &filename, &title, &path, &year, &dur, &size, &thumb,
			&backdrop, &overview, &imdb, &tmdb, &source, &container, &vcodec, &acodec, &created,
			&width, &height, &mtime)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		if uid, _ := c.Locals("uid").(string); !canAccessPath(db, uid, path.String) {
			return c.Status(403).JSON(fiber.Map{"err": "forbidden"})
		}
		m := videoMap(vid, filename, title, year, dur, size, thumb, created, mtime)
		m["path"] = path.String
		m["backdrop"] = backdrop.String
		m["overview"] = overview.String
		m["imdb_id"] = imdb.String
		m["tmdb_id"] = tmdb.String
		m["metadata_source"] = source.String
		m["container"] = container.String
		m["vcodec"] = vcodec.String
		m["acodec"] = acodec.String
		m["width"] = int(width.Float64)
		m["height"] = int(height.Float64)
		ext := strings.ToLower(filepath.Ext(path.String))
		plan := planStream(ext, vcodec.String, acodec.String, cfg.FFCaps.H264Enc)
		m["stream_type"] = plan.Mime
		m["stream_mode"] = plan.Mode.String()
		m["direct_play"] = plan.Mode == modeDirect
		return c.JSON(m)
	}
}

// searchHandler searches videos by title/filename.
func searchHandler(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, _ := c.Locals("uid").(string)
		q := strings.TrimSpace(c.Query("q", ""))
		if q == "" {
			return c.JSON(fiber.Map{"items": []any{}})
		}
		like := "%" + q + "%"
		sql := `
			SELECT id, filename, title, year, duration, size, thumb_path, created_at, mtime
			FROM videos
			WHERE (title LIKE ? OR filename LIKE ?) AND path NOT LIKE '%/.%'`
		args := []any{like, like}
		if expr, fargs := pathFilter(db, uid, "path"); expr != "" {
			sql += " AND " + expr
			args = append(args, fargs...)
		}
		sql += " ORDER BY filename ASC LIMIT 100"
		rows, err := db.Query(sql, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		return c.JSON(fiber.Map{"items": scanVideoItems(rows)})
	}
}

func scanVideoItems(rows *sql.Rows) []fiber.Map {
	items := []fiber.Map{}
	for rows.Next() {
		var id, filename, title, thumb, created sql.NullString
		var year, mtime sql.NullInt64
		var dur, size sql.NullFloat64
		if err := rows.Scan(&id, &filename, &title, &year, &dur, &size, &thumb, &created, &mtime); err != nil {
			continue
		}
		items = append(items, videoMap(id, filename, title, year, dur, size, thumb, created, mtime))
	}
	return items
}

func videoMap(id, filename, title sql.NullString, year sql.NullInt64, dur, size sql.NullFloat64, thumb, created sql.NullString, mtime sql.NullInt64) fiber.Map {
	display := title.String
	if display == "" {
		display = filename.String
	}
	return fiber.Map{
		"type":       "video",
		"id":         id.String,
		"filename":   filename.String,
		"title":      display,
		"year":       year.Int64,
		"duration":   dur.Float64,
		"size":       int64(size.Float64),
		"thumb":      thumbURL(id.String, thumb),
		"created_at": created.String,
		"mtime":      mtime.Int64,
	}
}

func intParam(c *fiber.Ctx, key string, def int) int {
	v := c.Query(key, "")
	if v == "" {
		return def
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 || n > 1000 {
		return def
	}
	return n
}
