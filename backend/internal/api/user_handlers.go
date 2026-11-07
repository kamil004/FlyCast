package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kamil/flycast/internal/auth"
)

func me(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid, ok := c.Locals("uid").(string)
		if !ok || uid == "" {
			return c.Status(401).JSON(fiber.Map{"err": "unauthorized"})
		}
		var username, display sql.NullString
		var isAdmin, isGuest int
		_ = db.QueryRow("SELECT username, display_name, is_admin, is_guest FROM users WHERE id=?", uid).
			Scan(&username, &display, &isAdmin, &isGuest)
		return c.JSON(fiber.Map{
			"id":           uid,
			"username":     username.String,
			"display_name": display.String,
			"is_admin":     isAdmin == 1,
			"is_guest":     isGuest == 1,
		})
	}
}

func listUsers(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query("SELECT id, username, display_name, is_admin, is_guest, COALESCE(restricted_libraries,0) FROM users ORDER BY username")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		defer rows.Close()
		users := []fiber.Map{}
		for rows.Next() {
			var id, username, display sql.NullString
			var isAdmin, isGuest, restricted int
			_ = rows.Scan(&id, &username, &display, &isAdmin, &isGuest, &restricted)
			users = append(users, fiber.Map{
				"id":           id.String,
				"username":     username.String,
				"display_name": display.String,
				"is_admin":     isAdmin == 1,
				"is_guest":     isGuest == 1,
				"restricted":   restricted == 1,
				"library_ids":  libraryIDsFor(db, id.String),
			})
		}
		return c.JSON(fiber.Map{"users": users})
	}
}

// libraryIDsFor returns the library ids explicitly assigned to a user.
func libraryIDsFor(db *sql.DB, uid string) []string {
	out := []string{}
	rows, err := db.Query("SELECT library_id FROM user_libraries WHERE user_id=?", uid)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// setUserLibraries replaces the explicit library assignment of a user.
func setUserLibraries(db *sql.DB, uid string, ids []string) {
	_, _ = db.Exec("DELETE FROM user_libraries WHERE user_id=?", uid)
	for _, id := range ids {
		if id == "" {
			continue
		}
		_, _ = db.Exec("INSERT OR IGNORE INTO user_libraries(user_id, library_id) VALUES(?,?)", uid, id)
	}
}

func createUser(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Username   string   `json:"username"`
			Password   string   `json:"password"`
			Display    string   `json:"display_name"`
			IsAdmin    bool     `json:"is_admin"`
			IsGuest    bool     `json:"is_guest"`
			Restricted bool     `json:"restricted"`
			LibraryIDs []string `json:"library_ids"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}
		if body.Username == "" || (body.Password == "" && !body.IsGuest) {
			return c.Status(400).JSON(fiber.Map{"err": "missing_fields"})
		}
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", body.Username).Scan(&cnt)
		if cnt > 0 {
			return c.Status(409).JSON(fiber.Map{"err": "username_exists"})
		}
		// Guests sign in via POST /api/auth/guest (no password); give them a
		// random secret so password login stays impossible.
		pw := body.Password
		if pw == "" || body.IsGuest {
			pw = uuid.NewString()
		}
		hash, err := auth.Hash(pw)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": "hash_failed"})
		}
		uid := uuid.NewString()
		isAdmin, isGuest, restricted := 0, 0, 0
		if body.IsAdmin {
			isAdmin = 1
		}
		if body.IsGuest {
			isGuest = 1
		}
		if body.Restricted {
			restricted = 1
		}
		display := body.Display
		if display == "" {
			display = body.Username
		}
		_, err = db.Exec(
			"INSERT INTO users(id, username, password_hash, display_name, is_admin, is_guest, restricted_libraries) VALUES(?,?,?,?,?,?,?)",
			uid, body.Username, hash, display, isAdmin, isGuest, restricted)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		if restricted == 1 {
			setUserLibraries(db, uid, body.LibraryIDs)
		}
		return c.JSON(fiber.Map{
			"id": uid, "username": body.Username, "display_name": display,
			"is_admin": body.IsAdmin, "is_guest": body.IsGuest,
			"restricted": body.Restricted, "library_ids": libraryIDsFor(db, uid),
		})
	}
}

func updateUser(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		self, _ := c.Locals("uid").(string)
		id := c.Params("id")
		if id == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_id"})
		}
		var body struct {
			Display    *string   `json:"display_name"`
			Password   *string   `json:"password"`
			IsAdmin    *bool     `json:"is_admin"`
			IsGuest    *bool     `json:"is_guest"`
			Restricted *bool     `json:"restricted"`
			LibraryIDs *[]string `json:"library_ids"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"err": "bad_request"})
		}

		var curAdmin, curGuest, curRestricted int
		if err := db.QueryRow("SELECT is_admin, is_guest, COALESCE(restricted_libraries,0) FROM users WHERE id=?", id).
			Scan(&curAdmin, &curGuest, &curRestricted); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}

		// Guard: an admin may not demote themselves (avoids locking out).
		if id == self && body.IsAdmin != nil && !*body.IsAdmin && curAdmin == 1 {
			return c.Status(400).JSON(fiber.Map{"err": "cannot_demote_self"})
		}
		// Guard: never remove the last admin.
		if body.IsAdmin != nil && !*body.IsAdmin && curAdmin == 1 {
			var admins int
			_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE is_admin=1").Scan(&admins)
			if admins <= 1 {
				return c.Status(400).JSON(fiber.Map{"err": "last_admin"})
			}
		}

		if body.Display != nil {
			_, _ = db.Exec("UPDATE users SET display_name=? WHERE id=?", *body.Display, id)
		}
		if body.Password != nil && *body.Password != "" {
			if hash, err := auth.Hash(*body.Password); err == nil {
				_, _ = db.Exec("UPDATE users SET password_hash=? WHERE id=?", hash, id)
			}
		}
		if body.IsAdmin != nil {
			_, _ = db.Exec("UPDATE users SET is_admin=? WHERE id=?", boolToInt(*body.IsAdmin), id)
		}
		if body.IsGuest != nil {
			_, _ = db.Exec("UPDATE users SET is_guest=? WHERE id=?", boolToInt(*body.IsGuest), id)
		}
		if body.Restricted != nil {
			_, _ = db.Exec("UPDATE users SET restricted_libraries=? WHERE id=?", boolToInt(*body.Restricted), id)
			if !*body.Restricted {
				// Unrestricted -> drop explicit assignments.
				_, _ = db.Exec("DELETE FROM user_libraries WHERE user_id=?", id)
			}
		}
		if body.LibraryIDs != nil {
			var restricted int
			_ = db.QueryRow("SELECT COALESCE(restricted_libraries,0) FROM users WHERE id=?", id).Scan(&restricted)
			if restricted == 1 {
				setUserLibraries(db, id, *body.LibraryIDs)
			}
		}

		var u, d sql.NullString
		var a, g, r int
		_ = db.QueryRow("SELECT username, display_name, is_admin, is_guest, COALESCE(restricted_libraries,0) FROM users WHERE id=?", id).
			Scan(&u, &d, &a, &g, &r)
		return c.JSON(fiber.Map{
			"id": id, "username": u.String, "display_name": d.String,
			"is_admin": a == 1, "is_guest": g == 1,
			"restricted": r == 1, "library_ids": libraryIDsFor(db, id),
		})
	}
}

func deleteUser(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		self, _ := c.Locals("uid").(string)
		id := c.Params("id")
		if id == "" {
			return c.Status(400).JSON(fiber.Map{"err": "missing_id"})
		}
		if id == self {
			return c.Status(400).JSON(fiber.Map{"err": "cannot_delete_self"})
		}
		var isAdmin int
		if err := db.QueryRow("SELECT is_admin FROM users WHERE id=?", id).Scan(&isAdmin); err != nil {
			return c.Status(404).JSON(fiber.Map{"err": "not_found"})
		}
		if isAdmin == 1 {
			var admins int
			_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE is_admin=1").Scan(&admins)
			if admins <= 1 {
				return c.Status(400).JSON(fiber.Map{"err": "last_admin"})
			}
		}
		// Remove the user's personal data (foreign keys are not cascaded).
		_, _ = db.Exec("DELETE FROM user_libraries WHERE user_id=?", id)
		_, _ = db.Exec("DELETE FROM album_items WHERE album_id IN (SELECT id FROM albums WHERE user_id=?)", id)
		_, _ = db.Exec("DELETE FROM albums WHERE user_id=?", id)
		_, _ = db.Exec("DELETE FROM user_progress WHERE user_id=?", id)
		if _, err := db.Exec("DELETE FROM users WHERE id=?", id); err != nil {
			return c.Status(500).JSON(fiber.Map{"err": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
