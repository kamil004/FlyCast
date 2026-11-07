package api

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/kamil/flycast/internal/auth"
	"github.com/kamil/flycast/internal/config"
)

func Setup(app *fiber.App, db *sql.DB, cfg *config.Config) {
	api := app.Group("/api")

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true, "name": "FlyCast", "ver": "0.1.0"})
	})

	api.Get("/auth/status", func(c *fiber.Ctx) error {
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&cnt)
		return c.JSON(fiber.Map{"hasUsers": cnt > 0})
	})

	api.Post("/auth/init", func(c *fiber.Ctx) error {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		if body.Username == "" || body.Password == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_fields"})
		}
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&cnt)
		if cnt > 0 {
			return c.Status(403).JSON(fiber.Map{"err": "already_initialized"})
		}
		hash, err := auth.Hash(body.Password)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": "hash_failed"})
		}
		uid := "u_admin"
		_, err = db.Exec("INSERT INTO users(id, username, password_hash, display_name, is_admin) VALUES(?,?,?,?,1)", uid, body.Username, hash, body.Username)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		t, _ := auth.Token(uid, body.Username)
		return c.JSON(fiber.Map{"token": t, "user": fiber.Map{"id": uid, "username": body.Username, "is_admin": true, "is_guest": false}})
	})

	api.Post("/auth/login", func(c *fiber.Ctx) error {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		var id, hash string
		var isAdmin, isGuest int
		err := db.QueryRow("SELECT id, password_hash, is_admin, is_guest FROM users WHERE username=?", body.Username).Scan(&id, &hash, &isAdmin, &isGuest)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"err": "invalid_credentials"})
		}
		if err := auth.Check(hash, body.Password); err != nil {
			return c.Status(401).JSON(fiber.Map{"err": "invalid_credentials"})
		}
		t, _ := auth.Token(id, body.Username)
		return c.JSON(fiber.Map{"token": t, "user": fiber.Map{"id": id, "username": body.Username, "is_admin": isAdmin == 1, "is_guest": isGuest == 1}})
	})

	// One-click login as the first existing guest account (no password).
	api.Post("/auth/guest", func(c *fiber.Ctx) error {
		var id, username string
		err := db.QueryRow("SELECT id, username FROM users WHERE is_guest=1 ORDER BY created_at LIMIT 1").Scan(&id, &username)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "no_guest"})
		}
		t, _ := auth.Token(id, username)
		return c.JSON(fiber.Map{"token": t, "user": fiber.Map{"id": id, "username": username, "is_admin": false, "is_guest": true}})
	})

	protected := api.Group("", AuthRequired(db), GuestReadOnly())
	protected.Get("/users/me", me(db))
	protected.Get("/library/browse", browseLibrary(db))
	protected.Get("/library/libraries", librariesHandler(db))
	protected.Get("/library/recent", recentHandler(db))
	protected.Get("/library/continue", continueHandler(db))
	protected.Get("/library/all", allVideosHandler(db))
	protected.Get("/library/search", searchHandler(db))
	protected.Get("/videos/:id", videoDetailHandler(db, cfg))
	protected.Get("/stream/:id", streamHandler(db, cfg))
	protected.Get("/thumb/:id", thumbHandler(db, cfg))
	protected.Get("/metadata/search", metadataSearch(db, cfg))
	protected.Post("/metadata/apply/:id", metadataApply(db, cfg))
	protected.Put("/videos/:id/meta", videoMetaEdit(db, cfg))
	protected.Post("/metadata/library/:folder/refresh", metadataLibraryRefresh(db, cfg))
	protected.Get("/metadata/job", metadataJobStatus(db, cfg))
	protected.Post("/metadata/job/cancel", metadataJobCancel(db, cfg))
	protected.Get("/albums", listAlbums(db))
	protected.Post("/albums", createAlbum(db))
	protected.Get("/albums/:id", getAlbum(db))
	protected.Delete("/albums/:id", deleteAlbum(db))
	protected.Post("/albums/:id/add", addToAlbum(db))
	protected.Post("/albums/:id/remove", removeFromAlbum(db))
	protected.Get("/favorites", listFavorites(db))
	protected.Get("/favorites/ids", favoriteIDs(db))
	protected.Post("/favorites/:id", addFavorite(db))
	protected.Delete("/favorites/:id", removeFavorite(db))
	protected.Get("/progress/:id", getProgress(db))
	protected.Post("/progress/:id", saveProgress(db))

	admin := protected.Group("", AdminRequired(db))
	admin.Get("/users", listUsers(db))
	admin.Post("/users", createUser(db))
	admin.Put("/users/:id", updateUser(db))
	admin.Delete("/users/:id", deleteUser(db))
	admin.Get("/libraries", listLibrariesConfig(db))
	admin.Post("/libraries", addLibrary(db, cfg))
	admin.Delete("/libraries/:id", removeLibrary(db))
	admin.Delete("/videos/:id", deleteVideo(db, cfg))
	admin.Post("/videos/:id/convert", convertVideoHandler(db, cfg))
	admin.Get("/convert/job", convertJobStatus(db, cfg))
	admin.Post("/convert/job/cancel", convertJobCancel(db, cfg))
	admin.Post("/libraries/scan", scanLibrariesAll(db))
	admin.Post("/libraries/:id/scan", scanLibraryOne(db))
	admin.Post("/libraries/:id/metadata/clear", metadataLibraryClear(db, cfg))
	admin.Get("/fs/browse", fsBrowse(db, cfg))

	if cfg.WebRoot != "" {
		app.Static("/", cfg.WebRoot)
		app.Get("/*", func(c *fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/api") {
				return c.Next()
			}
			return c.SendFile(cfg.WebRoot + "/index.html")
		})
	}
}
