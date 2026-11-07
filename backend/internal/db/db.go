package db

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  display_name TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  is_admin INTEGER DEFAULT 0,
  is_guest INTEGER DEFAULT 0,
  restricted_libraries INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS libraries (
  id TEXT PRIMARY KEY,
  path TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  media_type TEXT DEFAULT 'mixed',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_libraries (
  user_id TEXT NOT NULL,
  library_id TEXT NOT NULL,
  PRIMARY KEY(user_id, library_id),
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS folders (
  id TEXT PRIMARY KEY,
  path TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  parent_id TEXT,
  scanned_at TIMESTAMP,
  thumb_path TEXT,
  FOREIGN KEY(parent_id) REFERENCES folders(id)
);

CREATE TABLE IF NOT EXISTS videos (
  id TEXT PRIMARY KEY,
  folder_id TEXT,
  path TEXT UNIQUE NOT NULL,
  filename TEXT NOT NULL,
  title TEXT,
  year INTEGER,
	duration REAL,
	size INTEGER,
	mtime INTEGER,
	container TEXT,
	thumb_path TEXT,
	backdrop_path TEXT,
	overview TEXT,
	imdb_id TEXT,
	tmdb_id TEXT,
	metadata_source TEXT,
	vcodec TEXT,
	acodec TEXT,
	width INTEGER,
	height INTEGER,
	last_played TIMESTAMP,
	play_count INTEGER DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(folder_id) REFERENCES folders(id)
);

CREATE TABLE IF NOT EXISTS user_progress (
  user_id TEXT,
  video_id TEXT,
  position REAL DEFAULT 0,
  duration REAL DEFAULT 0,
  last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(user_id, video_id),
  FOREIGN KEY(user_id) REFERENCES users(id),
  FOREIGN KEY(video_id) REFERENCES videos(id)
);

CREATE TABLE IF NOT EXISTS albums (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  user_id TEXT,
  cover_video_id TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS album_items (
  album_id TEXT,
  video_id TEXT,
  sort_order INTEGER DEFAULT 0,
  PRIMARY KEY(album_id, video_id),
  FOREIGN KEY(album_id) REFERENCES albums(id),
  FOREIGN KEY(video_id) REFERENCES videos(id)
);

CREATE TABLE IF NOT EXISTS favorites (
  user_id TEXT NOT NULL,
  video_id TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(user_id, video_id),
  FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY(video_id) REFERENCES videos(id)
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT
);
`
	_, err := db.Exec(schema)
	if err != nil {
		log.Println("migrate err:", err)
		return err
	}
	// best-effort column migrations for existing databases
	_, _ = db.Exec("ALTER TABLE videos ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
	_, _ = db.Exec("ALTER TABLE videos ADD COLUMN vcodec TEXT")
	_, _ = db.Exec("ALTER TABLE videos ADD COLUMN acodec TEXT")
	_, _ = db.Exec("ALTER TABLE videos ADD COLUMN width INTEGER")
	_, _ = db.Exec("ALTER TABLE videos ADD COLUMN height INTEGER")
	_, _ = db.Exec("ALTER TABLE videos ADD COLUMN mtime INTEGER")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_guest INTEGER DEFAULT 0")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN restricted_libraries INTEGER DEFAULT 0")
	return nil
}
