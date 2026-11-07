package api

import (
	"database/sql"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// browseLibrary performs an Emby-style folder drill-down.
//   - no "folder" param      -> list root libraries
//   - "folder" = folder id    -> list child folders + videos of that folder
func browseLibrary(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		folder := strings.TrimSpace(c.Query("folder", ""))
		items := []fiber.Map{}
		uid, _ := c.Locals("uid").(string)
		unrestricted, roots := libAccess(db, uid)

		if folder == "" {
			rows, err := db.Query(folderCountSQL + ` WHERE f.parent_id IS NULL ORDER BY f.name`)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"err": err.Error()})
			}
			defer rows.Close()
			type rootRow struct {
				id, path, name, thumb string
				count                 int
				size                  int64
			}
			rootList := []rootRow{}
			for rows.Next() {
				var id, path, name, thumb sql.NullString
				var cnt int
				var sz sql.NullFloat64
				_ = rows.Scan(&id, &path, &name, &thumb, &cnt, &sz)
				if isHidden(name.String) {
					continue
				}
				if path.Valid && exists(path.String) {
					if !unrestricted && !pathIn(path.String, roots) {
						continue
					}
					rootList = append(rootList, rootRow{id.String, path.String, name.String, thumb.String, cnt, int64(sz.Float64)})
				}
			}
			rows.Close()
			for _, r := range rootList {
				items = append(items, fiber.Map{
					"type": "folder", "id": r.id, "path": r.path,
					"name": r.name, "thumb": r.thumb, "count": r.count, "size": r.size,
					"thumbs": libraryThumbs(db, r.id, 4),
				})
			}
			return c.JSON(fiber.Map{"items": items})
		}

		// Guard against drilling into a library the user cannot see.
		if !unrestricted {
			var fp sql.NullString
			if err := db.QueryRow("SELECT path FROM folders WHERE id=?", folder).Scan(&fp); err != nil || !pathIn(fp.String, roots) {
				return c.JSON(fiber.Map{"items": []fiber.Map{}, "folder": folder})
			}
		}

		// child folders
		frows, err := db.Query(folderCountSQL+` WHERE f.parent_id = ? ORDER BY f.name`, folder)
		if err == nil {
			defer frows.Close()
			for frows.Next() {
				var id, path, name, thumb sql.NullString
				var cnt int
				var sz sql.NullFloat64
				_ = frows.Scan(&id, &path, &name, &thumb, &cnt, &sz)
				if isHidden(name.String) {
					continue
				}
				if path.Valid && exists(path.String) {
					items = append(items, fiber.Map{
						"type": "folder", "id": id.String, "path": path.String,
						"name": name.String, "thumb": thumb.String, "count": cnt, "size": int64(sz.Float64),
						"thumbs": libraryThumbs(db, id.String, 4),
					})
				}
			}
		}

		// videos directly in this folder
		vrows, err := db.Query(`
			SELECT id, filename, title, year, duration, size, thumb_path, created_at, mtime
			FROM videos WHERE folder_id = ? ORDER BY filename
		`, folder)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer vrows.Close()
		for vrows.Next() {
			var id, filename, title, thumb, created sql.NullString
			var year, mtime sql.NullInt64
			var dur, size sql.NullFloat64
			if err := vrows.Scan(&id, &filename, &title, &year, &dur, &size, &thumb, &created, &mtime); err != nil {
				continue
			}
			items = append(items, videoMap(id, filename, title, year, dur, size, thumb, created, mtime))
		}

		root, mediaType := libraryOf(db, folder)
		return c.JSON(fiber.Map{"items": items, "folder": folder, "library_id": root,
			"media_type": mediaType, "crumbs": folderChain(db, folder)})
	}
}

// libraryOf walks a folder up to its root and returns the root folder id plus
// the configured media_type (movies/tv/mixed/home) of the matching library.
func libraryOf(db *sql.DB, folder string) (root, mediaType string) {
	root = folder
	path := ""
	for i := 0; i < 64; i++ {
		var pid, p sql.NullString
		if err := db.QueryRow("SELECT parent_id, path FROM folders WHERE id=?", root).Scan(&pid, &p); err != nil {
			break
		}
		path = p.String
		if !pid.Valid || pid.String == "" {
			break
		}
		root = pid.String
	}
	var mt sql.NullString
	_ = db.QueryRow(`SELECT media_type FROM libraries
		WHERE path = ? OR path = ? LIMIT 1`,
		path, strings.TrimRight(path, "/")).Scan(&mt)
	return root, mt.String
}

// folderChain returns a folder's ancestors from the library root down to the
// folder itself (inclusive), so the UI can render a full breadcrumb path.
func folderChain(db *sql.DB, folder string) []fiber.Map {
	type ent struct{ id, name string }
	var rev []ent
	cur := folder
	for i := 0; i < 64 && cur != ""; i++ {
		var id, name, pid sql.NullString
		if err := db.QueryRow("SELECT id, name, parent_id FROM folders WHERE id=?", cur).
			Scan(&id, &name, &pid); err != nil {
			break
		}
		rev = append(rev, ent{id.String, name.String})
		if !pid.Valid || pid.String == "" {
			break
		}
		cur = pid.String
	}
	out := make([]fiber.Map, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, fiber.Map{"id": rev[i].id, "name": rev[i].name})
	}
	return out
}

// folderCountSQL selects folders together with the recursive count and total
// size (bytes) of videos in their entire subtree (any depth). Callers append
// their own WHERE/ORDER BY.
const folderCountSQL = `
	WITH RECURSIVE
	  desc(ancestor, descendant) AS (
	    SELECT id, id FROM folders
	    UNION ALL
	    SELECT d.ancestor, f.id FROM folders f JOIN desc d ON f.parent_id = d.descendant
	  ),
	  vagg(folder_id, n, sz) AS (
	    SELECT folder_id, COUNT(*), IFNULL(SUM(size),0) FROM videos GROUP BY folder_id
	  )
	SELECT f.id, f.path, f.name, f.thumb_path,
	       (SELECT IFNULL(SUM(va.n),0) FROM desc d JOIN vagg va ON va.folder_id = d.descendant
	          WHERE d.ancestor = f.id) AS cnt,
	       (SELECT IFNULL(SUM(va.sz),0) FROM desc d JOIN vagg va ON va.folder_id = d.descendant
	          WHERE d.ancestor = f.id) AS sz
	FROM folders f`

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// isHidden reports whether a name should be hidden from the library UI
// (entries starting with a dot).
func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}
