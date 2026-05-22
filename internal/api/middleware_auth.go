package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"omsu_bot/internal/db"
)

type AuthMiddleware struct {
	jwtSecret  []byte
	adminToken string
	db         *sql.DB
}

func NewAuthMiddleware(adminSecret, jwtSecret string, database *sql.DB) *AuthMiddleware {
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

	if req.Secret != m.adminToken {
		return respondError(c, fiber.StatusUnauthorized, ErrInvalidRequest, "invalid admin_secret")
	}

	now := time.Now()
	// JWT TTL reduced to 4 hours (FIX-11). Use refresh flow if longer sessions needed.
	expiry := now.Add(4 * time.Hour)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(now),
			// jti is the wall-clock nanosecond — unique enough for this use case.
			ID: time.Now().Format("20060102150405.000000000"),
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
	})
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
		_ = d.RevokeToken(c.Context(), claims.ID, claims.ExpiresAt.Unix())
		_ = d.PruneRevokedTokens(c.Context())
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
	})

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
