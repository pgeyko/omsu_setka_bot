package api

import (
	"crypto/subtle"
	"database/sql"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"omsu_bot/internal/db"
)

type AuthMiddleware struct {
	jwtSecret  []byte
	adminToken string
	db         *sql.DB
}

func NewAuthMiddleware(adminSecret, jwtSecret string, database *sql.DB) *AuthMiddleware {
	if len(adminSecret) < 16 {
		slog.Warn("admin_secret is too short, minimum 16 characters recommended")
	}
	return &AuthMiddleware{
		jwtSecret:  []byte(jwtSecret),
		adminToken: adminSecret,
		db:         database,
	}
}

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

func (m *AuthMiddleware) Login(c *fiber.Ctx) error {
	var req struct {
		Secret string `json:"admin_secret"`
	}

	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid JSON body")
	}

	if subtle.ConstantTimeCompare([]byte(req.Secret), []byte(m.adminToken)) != 1 {
		return respondError(c, fiber.StatusUnauthorized, ErrInvalidRequest, "invalid admin_secret")
	}

	now := time.Now()
	// JWT TTL reduced to 4 hours (FIX-11). Use refresh flow if longer sessions needed.
	expiry := now.Add(4 * time.Hour)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(now),
		ID: uuid.NewString(),
		},
		Role: "admin",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.jwtSecret)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "token generation failed")
	}

	return respondSuccess(c, fiber.Map{
		"token":      signed,
		"expires_at": expiry.UTC().Format(time.RFC3339),
	})
}

func (m *AuthMiddleware) LoginWithRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return respondError(c, fiber.StatusTooManyRequests, ErrRateLimited, "too many login attempts")
		},
	})
}

func (m *AuthMiddleware) Logout(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return respondError(c, fiber.StatusUnauthorized, ErrUnauthorized, "missing Authorization header")
	}

	tokenStr := authHeader
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		tokenStr = tokenStr[7:]
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return m.jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		// Token already invalid — treat as logged out.
		return respondSuccess(c, fiber.Map{"status": "ok"})
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return respondSuccess(c, fiber.Map{"status": "ok"})
	}

	// Blacklist the jti until it would have expired anyway.
	if m.db != nil {
		d := &db.DB{DB: m.db}
		if err := d.RevokeToken(c.Context(), claims.ID, claims.ExpiresAt.Unix()); err != nil {
			slog.Warn("revoke token", "error", err)
		}
		if err := d.PruneRevokedTokens(c.Context()); err != nil {
			slog.Warn("prune revoked tokens", "error", err)
		}
	}

	return respondSuccess(c, fiber.Map{"status": "ok"})
}

func (m *AuthMiddleware) RequireAuth(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return respondError(c, fiber.StatusUnauthorized, ErrUnauthorized, "missing Authorization header")
	}

	tokenStr := authHeader
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		tokenStr = tokenStr[7:]
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return m.jwtSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil || !token.Valid {
		return respondError(c, fiber.StatusUnauthorized, ErrInvalidToken, "invalid or expired token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || claims.Role != "admin" {
		return respondError(c, fiber.StatusForbidden, ErrForbidden, "insufficient permissions")
	}

	// Check revocation blacklist.
	if m.db != nil && claims.ID != "" {
		d := &db.DB{DB: m.db}
		revoked, err := d.IsTokenRevoked(c.Context(), claims.ID)
		if err == nil && revoked {
			return respondError(c, fiber.StatusUnauthorized, ErrInvalidToken, "token has been revoked")
		}
	}

	return c.Next()
}
