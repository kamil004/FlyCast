package scanner

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kamil/flycast/internal/config"
)

var (
	reDuration = regexp.MustCompile(`Duration:\s*(\d+):(\d+):(\d+)\.(\d+)`)
	reVideoDim = regexp.MustCompile(`Video:.*?(\d{2,5})x(\d{2,5})`)
	reVCodec   = regexp.MustCompile(`Video:\s*([a-zA-Z0-9_]+)`)
	reACodec   = regexp.MustCompile(`Audio:\s*([a-zA-Z0-9_]+)`)
)

// MediaInfo holds technical metadata extracted from a media file.
type MediaInfo struct {
	Duration float64
	VCodec   string
	ACodec   string
	Width    int
	Height   int
}

// Probe inspects a media file using ffmpeg (ffprobe may be unavailable).
func Probe(ffmpeg, path string) MediaInfo {
	var mi MediaInfo
	cmd := exec.Command(ffmpeg, "-hide_banner", "-i", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // ffmpeg exits non-zero without an output target; we only need the probe text

	out := stderr.String()
	if m := reDuration.FindStringSubmatch(out); len(m) == 5 {
		h, _ := strconv.Atoi(m[1])
		mm, _ := strconv.Atoi(m[2])
		s, _ := strconv.Atoi(m[3])
		cs, _ := strconv.Atoi(m[4])
		mi.Duration = float64(h*3600+mm*60+s) + float64(cs)/100.0
	}
	if m := reVCodec.FindStringSubmatch(out); len(m) == 2 {
		mi.VCodec = strings.ToLower(m[1])
	}
	if m := reACodec.FindStringSubmatch(out); len(m) == 2 {
		mi.ACodec = strings.ToLower(m[1])
	}
	if m := reVideoDim.FindStringSubmatch(out); len(m) == 3 {
		mi.Width, _ = strconv.Atoi(m[1])
		mi.Height, _ = strconv.Atoi(m[2])
	}
	return mi
}

var exts = map[string]bool{
	".mp4": true, ".mkv": true, ".webm": true, ".avi": true, ".mov": true,
	".m4v": true, ".ts": true, ".mpg": true, ".mpeg": true, ".flv": true,
	".wmv": true, ".rmvb": true, ".ogv": true,
}

// scanReq carries a library id to scan, or "" to scan all libraries.
var scanReq = make(chan string, 16)

// Start seeds the libraries table from the configured paths (first run only),
// performs an initial scan, then processes scan requests serially.
func Start(db *sql.DB, cfg *config.Config) {
	seedLibraries(db, cfg)
	PruneHidden(db)
	go Watch(db, cfg, cfg.WatchInterval)
	go func() {
		backfillMtime(db)
		ScanAll(db, cfg)
		log.Println("scanner: initial scan done")
		for id := range scanReq {
			if id == "" {
				ScanAll(db, cfg)
			} else {
				ScanLibraryByID(db, cfg, id)
			}
			log.Println("scanner: rescan done")
		}
	}()
}

// Trigger requests a full rescan of all libraries (non-blocking).
func Trigger() {
	select {
	case scanReq <- "":
	default:
	}
}

// Watch periodically fingerprints each library tree and triggers a rescan of
// the affected library as soon as something changes (e.g. a new media file is
// copied in). Directory modification times are used as a cheap fingerprint:
// adding or removing an entry updates the mtime of its parent directory, so we
// avoid stat-ing every media file on each tick. interval <= 0 disables it.
func Watch(db *sql.DB, cfg *config.Config, interval time.Duration) {
	if interval <= 0 {
		return
	}
	last := map[string]string{}
	check := func() {
		rows, err := db.Query("SELECT id, path FROM libraries")
		if err != nil {
			return
		}
		type lib struct{ id, path string }
		libs := []lib{}
		for rows.Next() {
			var l lib
			if rows.Scan(&l.id, &l.path) == nil {
				libs = append(libs, l)
			}
		}
		rows.Close()
		for _, l := range libs {
			sig := dirSignature(l.path)
			if prev, ok := last[l.id]; ok && prev != sig {
				log.Printf("scanner: change detected in %q, rescanning", l.path)
				TriggerLibrary(l.id)
			}
			last[l.id] = sig
		}
	}
	check() // establish baseline without triggering
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		check()
	}
}

// dirSignature builds a fingerprint of a directory tree from the modification
// time of every (non-hidden) directory in it.
func dirSignature(root string) string {
	if root == "" {
		return ""
	}
	var b strings.Builder
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if fi, e := d.Info(); e == nil {
			b.WriteString(path)
			b.WriteByte(':')
			b.WriteString(strconv.FormatInt(fi.ModTime().UnixNano(), 10))
			b.WriteByte('\n')
		}
		return nil
	})
	return b.String()
}

// TriggerLibrary requests a rescan of a single library (non-blocking).
func TriggerLibrary(id string) {
	select {
	case scanReq <- id:
	default:
	}
}

func seedLibraries(db *sql.DB, cfg *config.Config) {
	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) FROM libraries").Scan(&cnt); err != nil {
		log.Println("scanner: count libraries err:", err)
		return
	}
	if cnt > 0 {
		return
	}
	for _, root := range cfg.Library {
		if root == "" {
			continue
		}
		name := filepath.Base(strings.TrimRight(root, "/"))
		if name == "" || name == "." || name == "/" {
			name = root
		}
		_, _ = db.Exec("INSERT OR IGNORE INTO libraries(id,path,name,media_type) VALUES(?,?,?,?)",
			uuid.NewString(), root, name, "mixed")
	}
}

// hiddenPathSQL matches any path with a directory component starting with a
// dot (e.g. QNAP's ".@__thumb", macOS ".AppleDouble"). Such entries are never
// shown by the scanner or the API.
const hiddenPathSQL = `path LIKE '%/.%'`

// PruneHidden removes videos and folders that live inside hidden (dot)
// directories. Older scans may have indexed them before hidden entries were
// skipped.
func PruneHidden(db *sql.DB) {
	sel := `SELECT id FROM videos WHERE ` + hiddenPathSQL
	_, _ = db.Exec(`DELETE FROM album_items WHERE video_id IN (` + sel + `)`)
	_, _ = db.Exec(`DELETE FROM user_progress WHERE video_id IN (` + sel + `)`)
	_, _ = db.Exec(`DELETE FROM favorites WHERE video_id IN (` + sel + `)`)
	if res, err := db.Exec(`DELETE FROM videos WHERE ` + hiddenPathSQL); err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("scanner: pruned %d hidden video(s)", n)
		}
	}
	if res, err := db.Exec(`DELETE FROM folders WHERE ` + hiddenPathSQL); err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("scanner: pruned %d hidden folder(s)", n)
		}
	}
}

// ScanAll scans every configured library.
func ScanAll(db *sql.DB, cfg *config.Config) {
	rows, err := db.Query("SELECT id, path FROM libraries ORDER BY name")
	if err != nil {
		log.Println("scanner: list libraries err:", err)
		return
	}
	type lib struct{ id, path string }
	libs := []lib{}
	for rows.Next() {
		var id, p string
		_ = rows.Scan(&id, &p)
		libs = append(libs, lib{id, p})
	}
	rows.Close()
	for _, l := range libs {
		scanLibraryPath(db, cfg, l.path)
	}
}

// ScanLibraryByID scans a single library by its id.
func ScanLibraryByID(db *sql.DB, cfg *config.Config, id string) {
	var path string
	if err := db.QueryRow("SELECT path FROM libraries WHERE id=?", id).Scan(&path); err != nil {
		log.Printf("scanner: library %s not found", id)
		return
	}
	PruneHidden(db)
	scanLibraryPath(db, cfg, path)
}

// RemoveLibrary deletes a library configuration together with all media,
// folders and progress rows that live under its path.
func RemoveLibrary(db *sql.DB, id string) error {
	var path string
	if err := db.QueryRow("SELECT path FROM libraries WHERE id=?", id).Scan(&path); err != nil {
		return err
	}
	prefix := strings.TrimRight(path, "/") + "/"
	under := `(v.path = ? OR substr(v.path,1,length(?)) = ?)`
	// remove dependent rows first
	_, _ = db.Exec(`DELETE FROM album_items WHERE video_id IN
		(SELECT id FROM videos v WHERE `+under+`)`, path, prefix, prefix)
	_, _ = db.Exec(`DELETE FROM user_progress WHERE video_id IN
		(SELECT id FROM videos v WHERE `+under+`)`, path, prefix, prefix)
	_, _ = db.Exec(`DELETE FROM favorites WHERE video_id IN
		(SELECT id FROM videos v WHERE `+under+`)`, path, prefix, prefix)
	_, _ = db.Exec(`DELETE FROM videos WHERE (path = ? OR substr(path,1,length(?)) = ?)`, path, prefix, prefix)
	_, _ = db.Exec(`DELETE FROM folders WHERE (path = ? OR substr(path,1,length(?)) = ?)`, path, prefix, prefix)
	_, err := db.Exec("DELETE FROM libraries WHERE id=?", id)
	return err
}

func scanLibraryPath(db *sql.DB, cfg *config.Config, root string) {
	if root == "" {
		return
	}
	if _, err := os.Stat(root); err != nil {
		log.Printf("scanner: library path not found: %s", root)
		return
	}
	scanDir(db, cfg, root, nil)
}

func scanDir(db *sql.DB, cfg *config.Config, dir string, parentID *string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	fid := ensureFolder(db, dir, parentID)

	for _, e := range ents {
		// Skip hidden entries (dotfiles/dotfolders) such as macOS ".DS_Store",
		// QNAP "@eaDir" style metadata handled elsewhere, and user-hidden dirs.
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			scanDir(db, cfg, p, &fid)
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !exts[ext] {
			continue
		}
		addVideo(db, cfg, fid, p)
	}

	db.Exec("UPDATE folders SET scanned_at=CURRENT_TIMESTAMP WHERE id=?", fid)
}

func ensureFolder(db *sql.DB, path string, parentID *string) string {
	var id string
	err := db.QueryRow("SELECT id FROM folders WHERE path=?", path).Scan(&id)
	if err == nil {
		// keep the parent in sync (nil for library roots)
		var pid interface{}
		if parentID != nil {
			pid = *parentID
		}
		db.Exec("UPDATE folders SET parent_id=? WHERE id=?", pid, id)
		return id
	}
	id = uuid.NewString()
	name := filepath.Base(path)
	var pid interface{}
	if parentID != nil {
		pid = *parentID
	}
	_, _ = db.Exec("INSERT INTO folders(id,path,name,parent_id) VALUES(?,?,?,?)", id, path, name, pid)
	return id
}

func addVideo(db *sql.DB, cfg *config.Config, folderID, path string) {
	var vid string
	var dur sql.NullFloat64
	err := db.QueryRow("SELECT id, duration FROM videos WHERE path=?", path).Scan(&vid, &dur)
	if err == nil {
		if !dur.Valid || dur.Float64 == 0 {
			updateMediaInfo(db, cfg, vid, path)
		}
		_ = ensureVideoThumb(db, cfg, vid, path)
		return
	}
	vid = uuid.NewString()
	filename := filepath.Base(path)
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	fi, _ := os.Stat(path)
	size := int64(0)
	var mtime interface{}
	if fi != nil {
		size = fi.Size()
		mtime = fi.ModTime().Unix()
	}

	_, err = db.Exec(`
		INSERT INTO videos(id,folder_id,path,filename,title,size,mtime)
		VALUES(?,?,?,?,?,?,?)
	`, vid, folderID, path, filename, title, size, mtime)
	if err != nil {
		log.Printf("scanner: insert video err: %v", err)
		return
	}
	updateMediaInfo(db, cfg, vid, path)
	_ = ensureVideoThumb(db, cfg, vid, path)
}

func updateMediaInfo(db *sql.DB, cfg *config.Config, vid, path string) {
	mi := Probe(cfg.FFmpeg, path)
	_, _ = db.Exec(`
		UPDATE videos SET duration=?, vcodec=?, acodec=?, width=?, height=?
		WHERE id=?
	`, mi.Duration, mi.VCodec, mi.ACodec, mi.Width, mi.Height, vid)
}

// backfillMtime fills the file-modification-time column for rows scanned before
// the column existed, so "sort by file date" works without a full rescan.
func backfillMtime(db *sql.DB) {
	rows, err := db.Query("SELECT id, path FROM videos WHERE mtime IS NULL")
	if err != nil {
		return
	}
	type item struct{ id, path string }
	var items []item
	for rows.Next() {
		var it item
		if rows.Scan(&it.id, &it.path) == nil {
			items = append(items, it)
		}
	}
	rows.Close()
	for _, it := range items {
		if fi, err := os.Stat(it.path); err == nil {
			_, _ = db.Exec("UPDATE videos SET mtime=? WHERE id=?", fi.ModTime().Unix(), it.id)
		}
	}
}

func thumbSeekTime(duration float64) string {
	if duration <= 0 {
		return "00:00:01.000"
	}
	ts := duration * 0.1
	if ts > 5 {
		ts = 5
	}
	return fmt.Sprintf("%.3f", ts)
}

func ensureVideoThumb(db *sql.DB, cfg *config.Config, vid, path string) error {
	out := filepath.Join(cfg.DataDir, "thumbs", vid+".jpg")
	if _, err := os.Stat(out); err == nil {
		db.Exec("UPDATE videos SET thumb_path=? WHERE id=?", out, vid)
		return nil
	}
	// Seek to 10% of the clip (capped at 5s); a fixed 5s yields no frame for
	// short clips.
	var dur sql.NullFloat64
	_ = db.QueryRow("SELECT duration FROM videos WHERE id=?", vid).Scan(&dur)
	ss := thumbSeekTime(dur.Float64)
	cmd := exec.Command(cfg.FFmpeg, "-y", "-ss", ss, "-i", path, "-vframes", "1", "-vf", "scale='min(480,iw)':-2", "-q:v", "4", out)
	_ = cmd.Run()
	if _, err := os.Stat(out); err == nil {
		_, _ = db.Exec("UPDATE videos SET thumb_path=? WHERE id=?", out, vid)
	}
	return nil
}
