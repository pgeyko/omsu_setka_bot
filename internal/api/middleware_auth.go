package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	jwtSecret  []byte
	adminToken string
}

func NewAuthMiddleware(adminSecret, jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:  []byte(jwtSecret),
		adminToken: adminSecret,
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

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Role: "admin",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.jwtSecret)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "token generation failed")
	}

	return respondSuccess(c, fiber.Map{"token": signed})
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

	return c.Next()
}
