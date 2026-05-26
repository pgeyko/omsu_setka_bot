package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"omsu_bot/internal/db"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func setupAuthTest(t *testing.T) (*AuthMiddleware, *db.DB, func()) {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	mw := NewAuthMiddleware("test-admin-secret-16chars", "test-jwt-secret", database.DB)
	cleanup := func() { database.Close() }
	return mw, database, cleanup
}

func generateToken(secret string, claims Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

func TestAuthMiddleware_Login_Valid(t *testing.T) {
	t.Parallel()
	mw, _, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/login", mw.Login)

	body := `{"admin_secret": "test-admin-secret-16chars"}`
	req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		t.Fatal("login should succeed")
	}
	if result.Data.Token == "" {
		t.Fatal("token should not be empty")
	}
	if result.Data.ExpiresAt == "" {
		t.Fatal("expires_at should not be empty")
	}
}

func TestAuthMiddleware_Login_InvalidSecret(t *testing.T) {
	t.Parallel()
	mw, _, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/login", mw.Login)

	body := `{"admin_secret": "wrong-secret"}`
	req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_Login_InvalidBody(t *testing.T) {
	t.Parallel()
	mw, _, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/login", mw.Login)

	req := httptest.NewRequest("POST", "/login", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_RequireAuth_ValidToken(t *testing.T) {
	t.Parallel()
	mw, _, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/protected", mw.RequireAuth, func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := generateToken("test-jwt-secret", Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			ID:        "test-jti-1",
		},
		Role: "admin",
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_RequireAuth_MissingToken(t *testing.T) {
	t.Parallel()
	mw, _, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/protected", mw.RequireAuth, func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_RequireAuth_ExpiredToken(t *testing.T) {
	t.Parallel()
	mw, _, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/protected", mw.RequireAuth, func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := generateToken("test-jwt-secret", Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			ID:        "test-jti-2",
		},
		Role: "admin",
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_RequireAuth_WrongRole(t *testing.T) {
	t.Parallel()
	mw, _, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/protected", mw.RequireAuth, func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := generateToken("test-jwt-secret", Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			ID:        "test-jti-3",
		},
		Role: "user",
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_Logout_RevokesToken(t *testing.T) {
	t.Parallel()
	mw, database, cleanup := setupAuthTest(t)
	defer cleanup()

	app := fiber.New()
	app.Post("/logout", mw.Logout)

	// Generate a valid token
	token := generateToken("test-jwt-secret", Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			ID:        "revocable-jti",
		},
		Role: "admin",
	})

	req := httptest.NewRequest("POST", "/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify the token is revoked in the database
	var count int
	database.QueryRow("SELECT COUNT(*) FROM revoked_tokens WHERE jti = 'revocable-jti'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 revoked token, got %d", count)
	}
}
