package api

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kamil/flycast/internal/config"
	"github.com/kamil/flycast/internal/scanner"
)

// fsRoots are the top level directories the folder picker is allowed to browse.
var fsRoots = []string{"/share", "/volume1", "/volume2", "/volume3", "/volume4", "/mnt"}

// listLibrariesConfig returns the configured libraries together with their
// matched root folder id and media count.
func listLibrariesConfig(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(`
			SELECT l.id, l.path, l.name, l.media_type, l.created_at, f.id AS folder_id,
			  (SELECT COUNT(*) FROM videos v
			     WHERE v.path = l.path
			        OR substr(v.path,1,length(l.path)+1) = l.path || '/') AS cnt
			FROM libraries l
			LEFT JOIN folders f ON f.path = l.path
			ORDER BY l.name
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		items := []fiber.Map{}
		for rows.Next() {
			var id, path, name, mtype, created, folderID sql.NullString
			var cnt int
			_ = rows.Scan(&id, &path, &name, &mtype, &created, &folderID, &cnt)
			items = append(items, fiber.Map{
				"id":         id.String,
				"path":       path.String,
				"name":       name.String,
				"media_type": mtype.String,
				"created_at": created.String,
				"folder_id":  folderID.String,
				"count":      cnt,
				"exists":     exists(path.String),
			})
		}
		return c.JSON(fiber.Map{"items": items})
	}
}

func addLibrary(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Path      string `json:"path"`
			Name      string `json:"name"`
			MediaType string `json:"media_type"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		path := strings.TrimRight(strings.TrimSpace(body.Path), "/")
		if path == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_path"})
		}
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			return c.Status(400).JSON(fiber.Map{"err": "path_not_found"})
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			name = filepath.Base(path)
		}
		mtype := body.MediaType
		if mtype == "" {
			mtype = "mixed"
		}

		var existing string
		if err := db.QueryRow("SELECT id FROM libraries WHERE path=?", path).Scan(&existing); err == nil {
			return c.Status(409).JSON(fiber.Map{"err": "already_exists", "id": existing})
		}

		id := uuid.NewString()
		if _, err := db.Exec("INSERT INTO libraries(id,path,name,media_type) VALUES(?,?,?,?)", id, path, name, mtype); err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		scanner.TriggerLibrary(id)
		return c.JSON(fiber.Map{"id": id, "path": path, "name": name, "media_type": mtype, "count": 0})
	}
}

func removeLibrary(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := scanner.RemoveLibrary(db, id); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func scanLibrariesAll(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		scanner.Trigger()
		return c.JSON(fiber.Map{"ok": true})
	}
}

func scanLibraryOne(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM libraries WHERE id=?", id).Scan(&cnt)
		if cnt == 0 {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		scanner.TriggerLibrary(id)
		return c.JSON(fiber.Map{"ok": true})
	}
}

// fsBrowse lists sub-directories for the library folder picker.
func fsBrowse(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqPath := strings.TrimSpace(c.Query("path", ""))

		type entry struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		dirs := []entry{}

		if reqPath == "" {
			for _, r := range fsRoots {
				if fi, err := os.Stat(r); err == nil && fi.IsDir() {
					dirs = append(dirs, entry{Name: r, Path: r})
				}
			}
			return c.JSON(fiber.Map{"path": "", "parent": "", "roots": true, "dirs": dirs})
		}

		clean := filepath.Clean(reqPath)
		if !allowedPath(clean) {
			return c.Status(403).JSON(fiber.Map{"err": "forbidden_path"})
		}
		ents, err := os.ReadDir(clean)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "cannot_read", "detail": err.Error()})
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			n := e.Name()
			if strings.HasPrefix(n, ".") || n == "@Recycle" || n == "#recycle" || n == "@Recently-Snapshot" {
				continue
			}
			dirs = append(dirs, entry{Name: n, Path: filepath.Join(clean, n)})
		}
		sort.Slice(dirs, func(i, j int) bool {
			return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
		})

		parent := filepath.Dir(clean)
		if !allowedPath(parent) || parent == clean {
			parent = ""
		}
		return c.JSON(fiber.Map{"path": clean, "parent": parent, "roots": false, "dirs": dirs})
	}
}

func allowedPath(p string) bool {
	p = filepath.Clean(p)
	for _, r := range fsRoots {
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// deleteVideo removes a video: its database row, dependent rows, the media file
// on disk and its thumbnails. Admin-only (wired into the admin route group).
func deleteVideo(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var path, thumb sql.NullString
		if err := db.QueryRow("SELECT path, thumb_path FROM videos WHERE id=?", id).Scan(&path, &thumb); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		_, _ = db.Exec("DELETE FROM album_items WHERE video_id=?", id)
		_, _ = db.Exec("DELETE FROM favorites WHERE video_id=?", id)
		_, _ = db.Exec("DELETE FROM user_progress WHERE video_id=?", id)
		if _, err := db.Exec("DELETE FROM videos WHERE id=?", id); err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		if path.Valid && path.String != "" {
			_ = os.Remove(path.String)
		}
		if thumb.Valid && thumb.String != "" {
			_ = os.Remove(thumb.String)
		}
		_ = os.Remove(filepath.Join(cfg.DataDir, "thumbs", id+".jpg"))
		return c.JSON(fiber.Map{"ok": true})
	}
}
