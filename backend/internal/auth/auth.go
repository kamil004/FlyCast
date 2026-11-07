package auth

import (
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// jwtSecretValue holds the runtime JWT signing secret. It is loaded from the
// FLYCAST_JWT_SECRET environment variable or generated randomly on startup.
var (
	jwtSecretValue []byte
	jwtSecretOnce  sync.Once
)

// InitJWTSecret must be called once at application startup (e.g. from main).
// It reads FLYCAST_JWT_SECRET from the environment; if unset, a 32-byte
// random secret is generated and printed to stderr so the operator knows its
// value for persistence across restarts.
func InitJWTSecret() {
	jwtSecretOnce.Do(func() {
		if s := os.Getenv("FLYCAST_JWT_SECRET"); s != "" {
			jwtSecretValue = []byte(s)
		} else {
			b := make([]byte, 32)
			_, _ = rand.Read(b)
			jwtSecretValue = b
			fmt.Fprintf(os.Stderr, "WARNING: FLYCAST_JWT_SECRET not set — generated random secret %x. Set it to keep the same tokens across restarts.\n", b)
		}
	})
}

// JwtSecret returns the current signing secret.
func JwtSecret() []byte {
	return jwtSecretValue
}

type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"un"`
	jwt.RegisteredClaims
}

func Hash(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

func Check(h, p string) error {
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(p))
}

func Token(uid, un string) (string, error) {
	claims := Claims{
		UserID:   uid,
		Username: un,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 30 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(JwtSecret())
}

func NewUserID() string {
	return uuid.NewString()
}
