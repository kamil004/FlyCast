package api

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kamil/flycast/internal/auth"
)

// AuthRequired validates the JWT (header or ?token=) and loads the caller's
// role into Locals: "uid", "username", "admin" (bool), "guest" (bool).
func AuthRequired(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := ""
		h := c.Get("Authorization")
		if strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		} else if q := c.Query("token"); q != "" {
			// <video> and <img> tags cannot send the Authorization header, so
			// media URLs (/api/stream, /api/thumb) may pass ?token=<jwt>.
			token = q
		}
		if token == "" {
			return c.Status(401).JSON(fiber.Map{"err": "unauthorized"})
		}
		var claims auth.Claims
		t, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(auth.JwtSecret()), nil
		})
		if err != nil || !t.Valid {
			return c.Status(401).JSON(fiber.Map{"err": "invalid_token"})
		}

		var isAdmin, isGuest int
		if err := db.QueryRow(
			"SELECT is_admin, is_guest FROM users WHERE id=?", claims.UserID,
		).Scan(&isAdmin, &isGuest); err != nil {
			return c.Status(401).JSON(fiber.Map{"err": "invalid_token"})
		}

		c.Locals("uid", claims.UserID)
		c.Locals("username", claims.Username)
		c.Locals("admin", isAdmin == 1)
		c.Locals("guest", isGuest == 1)
		return c.Next()
	}
}

// GuestReadOnly blocks every state-changing request for guest accounts, which
// may only browse and watch. It must run after AuthRequired.
func GuestReadOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if g, _ := c.Locals("guest").(bool); g {
			m := c.Method()
			if m != fiber.MethodGet && m != fiber.MethodHead {
				// Guests may still save their own playback progress so that
				// "resume where you left off" works for them too.
				if m == fiber.MethodPost && strings.HasPrefix(c.Path(), "/api/progress/") {
					return c.Next()
				}
				return c.Status(403).JSON(fiber.Map{"err": "guest_readonly"})
			}
		}
		return c.Next()
	}
}

// AdminRequired rejects non-admin users. It uses the role loaded by
// AuthRequired, falling back to a database lookup when absent.
func AdminRequired(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if a, ok := c.Locals("admin").(bool); ok {
			if a {
				return c.Next()
			}
			return c.Status(403).JSON(fiber.Map{"err": "forbidden"})
		}
		uid, _ := c.Locals("uid").(string)
		var isAdmin int
		if err := db.QueryRow("SELECT is_admin FROM users WHERE id=?", uid).Scan(&isAdmin); err != nil || isAdmin != 1 {
			return c.Status(403).JSON(fiber.Map{"err": "forbidden"})
		}
		return c.Next()
	}
}
