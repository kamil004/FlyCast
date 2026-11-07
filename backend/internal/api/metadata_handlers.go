package api

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kamil/flycast/internal/config"
	"github.com/kamil/flycast/internal/metadata"
)

func metadataSearch(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if imdb := strings.TrimSpace(c.Query("imdb", "")); imdb != "" {
			m, err := metadata.ByIMDB(imdb)
			if err != nil {
				return c.JSON(fiber.Map{"results": []any{}})
			}
			return c.JSON(fiber.Map{"results": []any{m}})
		}
		q := strings.TrimSpace(c.Query("q", ""))
		if q == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_q"})
		}
		source := strings.TrimSpace(c.Query("source", "imdb"))
		m, err := metadata.Search(source, q)
		if err != nil {
			return c.JSON(fiber.Map{"results": []any{}})
		}
		return c.JSON(fiber.Map{"results": []any{m}})
	}
}

// validImageURL reports whether a string is a safe, externally hosted image URL.
func validImageURL(u string) bool {
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	return !strings.ContainsAny(u, " \t\n\r\"'<>")
}

// savePoster downloads an image and stores it as the video's local thumbnail so
// the cover shows everywhere (cards, details, collages). Returns the path on
// success.
func savePoster(cfg *config.Config, id, url string) string {
	if !validImageURL(url) {
		return ""
	}
	data, err := metadata.FetchImage(url)
	if err != nil || len(data) == 0 {
		return ""
	}
	dir := filepath.Join(cfg.DataDir, "thumbs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	out := filepath.Join(dir, id+".jpg")
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return ""
	}
	return out
}

// applyMetadata writes scraped metadata for a video. Empty fields keep their
// previous value; when a poster is available it is downloaded as the thumbnail.
func applyMetadata(db *sql.DB, cfg *config.Config, id string, m metadata.VideoMeta) error {
	var cur metadata.VideoMeta
	_ = db.QueryRow(`
		SELECT IFNULL(title,''), IFNULL(year,0), IFNULL(overview,''),
		       IFNULL(imdb_id,''), IFNULL(tmdb_id,''), IFNULL(metadata_source,'')
		FROM videos WHERE id=?`, id).
		Scan(&cur.Title, &cur.Year, &cur.Overview, &cur.IMDBID, &cur.TMDBID, &cur.Source)

	if m.Title == "" {
		m.Title = cur.Title
	}
	if m.Year == 0 {
		m.Year = cur.Year
	}
	if m.Overview == "" {
		m.Overview = cur.Overview
	}
	if m.IMDBID == "" {
		m.IMDBID = cur.IMDBID
	}
	if m.TMDBID == "" {
		m.TMDBID = cur.TMDBID
	}
	if m.Source == "" {
		m.Source = cur.Source
	}

	if _, err := db.Exec(`
		UPDATE videos
		SET title=?, year=?, overview=?, imdb_id=?, tmdb_id=?, metadata_source=?
		WHERE id=?`,
		m.Title, m.Year, m.Overview, m.IMDBID, m.TMDBID, m.Source, id); err != nil {
		return err
	}

	poster := m.PosterPath
	if poster == "" {
		poster = m.BackdropPath
	}
	if p := savePoster(cfg, id, poster); p != "" {
		_, _ = db.Exec("UPDATE videos SET thumb_path=? WHERE id=?", p, id)
	}

	art := m.BackdropPath
	if art == "" {
		art = m.PosterPath
	}
	if validImageURL(art) {
		_, _ = db.Exec("UPDATE videos SET backdrop_path=? WHERE id=?", art, id)
	}
	return nil
}

// metadataApply handles the single-item scrape triggered from the detail page.
func metadataApply(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Title        string `json:"title"`
			Year         int    `json:"year"`
			Overview     string `json:"overview"`
			IMDBID       string `json:"imdb_id"`
			TMDBID       string `json:"tmdb_id"`
			PosterPath   string `json:"poster_path"`
			BackdropPath string `json:"backdrop_path"`
			Source       string `json:"source"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		err := applyMetadata(db, cfg, id, metadata.VideoMeta{
			Title: body.Title, Year: body.Year, Overview: body.Overview,
			IMDBID: body.IMDBID, TMDBID: body.TMDBID,
			PosterPath: body.PosterPath, BackdropPath: body.BackdropPath,
			Source: body.Source,
		})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// videoMetaEdit lets a user manually correct a video's metadata. Unlike
// applyMetadata it sets fields verbatim (so they can be cleared).
func videoMetaEdit(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Title     string `json:"title"`
			Year      int    `json:"year"`
			Overview  string `json:"overview"`
			IMDBID    string `json:"imdb_id"`
			TMDBID    string `json:"tmdb_id"`
			PosterURL string `json:"poster_url"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		title := strings.TrimSpace(body.Title)
		if title == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_title"})
		}
		_, err := db.Exec(`
			UPDATE videos SET title=?, year=?, overview=?, imdb_id=?, tmdb_id=?, metadata_source='manual' WHERE id=?`,
			title, body.Year, body.Overview, strings.TrimSpace(body.IMDBID), strings.TrimSpace(body.TMDBID), id)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		if strings.TrimSpace(body.PosterURL) != "" {
			if p := savePoster(cfg, id, strings.TrimSpace(body.PosterURL)); p != "" {
				_, _ = db.Exec("UPDATE videos SET thumb_path=?, backdrop_path=? WHERE id=?", p, body.PosterURL, id)
			} else {
				return c.Status(400).JSON(fiber.Map{"err": "poster_download_failed"})
			}
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// ------------------------- bulk metadata refresh ---------------------------

type metaJob struct {
	mu      sync.Mutex
	running bool
	cancel  bool
	done    int
	total   int
	updated int
	failed  int
	source  string
}

var bulkMetaJob = &metaJob{}

// videosInFolder returns the ids and best-guess titles of all videos in a
// folder subtree.
func videosInFolder(db *sql.DB, folder string) (ids, titles []string) {
	const q = `
WITH RECURSIVE desc(id) AS (
  SELECT id FROM folders WHERE id = ?
  UNION ALL
  SELECT f.id FROM folders f JOIN desc d ON f.parent_id = d.id
)
SELECT v.id, IFNULL(NULLIF(v.title,''), v.filename)
FROM videos v
WHERE v.folder_id IN (SELECT id FROM desc)
  AND v.path NOT LIKE '%/.%'`
	rows, err := db.Query(q, folder)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()
	for rows.Next() {
		var id, title string
		if rows.Scan(&id, &title) == nil {
			ids = append(ids, id)
			titles = append(titles, title)
		}
	}
	return ids, titles
}

// metadataLibraryRefresh starts a background job that scrapes metadata for every
// video in a library.
func metadataLibraryRefresh(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		folder := c.Params("folder")
		var body struct {
			Source string `json:"source"`
		}
		_ = c.BodyParser(&body)
		src := strings.ToLower(strings.TrimSpace(body.Source))
		if src != "filmweb" {
			src = "imdb"
		}

		bulkMetaJob.mu.Lock()
		if bulkMetaJob.running {
			bulkMetaJob.mu.Unlock()
			return c.Status(409).JSON(fiber.Map{"err": "busy"})
		}
		ids, titles := videosInFolder(db, folder)
		bulkMetaJob.running = true
		bulkMetaJob.cancel = false
		bulkMetaJob.done = 0
		bulkMetaJob.total = len(ids)
		bulkMetaJob.updated = 0
		bulkMetaJob.failed = 0
		bulkMetaJob.source = src
		bulkMetaJob.mu.Unlock()

		go runBulkMetadata(db, cfg, src, ids, titles)
		return c.JSON(fiber.Map{"started": true, "total": len(ids), "source": src})
	}
}

func runBulkMetadata(db *sql.DB, cfg *config.Config, source string, ids, titles []string) {
	log.Printf("metadata: bulk refresh start source=%s total=%d", source, len(ids))
	for i, id := range ids {
		bulkMetaJob.mu.Lock()
		if bulkMetaJob.cancel {
			bulkMetaJob.running = false
			bulkMetaJob.mu.Unlock()
			log.Println("metadata: bulk refresh cancelled")
			return
		}
		bulkMetaJob.mu.Unlock()

		ok := false
		if m, err := metadata.Search(source, titles[i]); err == nil && m != nil {
			if applyMetadata(db, cfg, id, *m) == nil {
				ok = true
			}
		}

		bulkMetaJob.mu.Lock()
		bulkMetaJob.done++
		if ok {
			bulkMetaJob.updated++
		} else {
			bulkMetaJob.failed++
		}
		bulkMetaJob.mu.Unlock()

		time.Sleep(250 * time.Millisecond)
	}
	bulkMetaJob.mu.Lock()
	bulkMetaJob.running = false
	bulkMetaJob.mu.Unlock()
	log.Printf("metadata: bulk refresh done updated=%d failed=%d", bulkMetaJob.updated, bulkMetaJob.failed)
}

func metadataJobStatus(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		bulkMetaJob.mu.Lock()
		defer bulkMetaJob.mu.Unlock()
		return c.JSON(fiber.Map{
			"running": bulkMetaJob.running,
			"done":    bulkMetaJob.done,
			"total":   bulkMetaJob.total,
			"updated": bulkMetaJob.updated,
			"failed":  bulkMetaJob.failed,
			"source":  bulkMetaJob.source,
		})
	}
}

func metadataJobCancel(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		bulkMetaJob.mu.Lock()
		if bulkMetaJob.running {
			bulkMetaJob.cancel = true
		}
		bulkMetaJob.mu.Unlock()
		return c.JSON(fiber.Map{"ok": true})
	}
}

// metadataLibraryClear wipes scraped metadata for every video in a library,
// restoring the filename-derived title and dropping downloaded posters so the
// scanner regenerates frame thumbnails.
func metadataLibraryClear(db *sql.DB, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		libID := c.Params("id")
		var libPath string
		if err := db.QueryRow("SELECT path FROM libraries WHERE id=?", libID).Scan(&libPath); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "library_not_found"})
		}
		var rootID string
		_ = db.QueryRow("SELECT id FROM folders WHERE path=? OR path=? LIMIT 1",
			libPath, strings.TrimRight(libPath, "/")).Scan(&rootID)
		if rootID == "" {
			return c.JSON(fiber.Map{"ok": true, "count": 0})
		}

		const q = `
WITH RECURSIVE desc(id) AS (
  SELECT id FROM folders WHERE id = ?
  UNION ALL
  SELECT f.id FROM folders f JOIN desc d ON f.parent_id = d.id
)
SELECT v.id, v.filename, IFNULL(v.thumb_path,'')
FROM videos v WHERE v.folder_id IN (SELECT id FROM desc)`
		rows, err := db.Query(q, rootID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		type row struct{ id, filename, thumb string }
		var list []row
		for rows.Next() {
			var r row
			if rows.Scan(&r.id, &r.filename, &r.thumb) == nil {
				list = append(list, r)
			}
		}
		rows.Close()

		count := 0
		for _, r := range list {
			stem := strings.TrimSuffix(r.filename, filepath.Ext(r.filename))
			if _, err := db.Exec(`
				UPDATE videos SET title=?, year=NULL, overview='', imdb_id='', tmdb_id='',
				  metadata_source='', backdrop_path='', thumb_path=NULL WHERE id=?`, stem, r.id); err != nil {
				continue
			}
			if r.thumb != "" && strings.HasPrefix(r.thumb, cfg.DataDir) {
				_ = os.Remove(r.thumb)
			}
			count++
		}
		return c.JSON(fiber.Map{"ok": true, "count": count})
	}
}
