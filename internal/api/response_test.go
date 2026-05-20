package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupTestApp() (*fiber.App, *Server) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	s := &Server{App: app}
	return app, s
}

func TestParsePagination_Defaults(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test", func(c *fiber.Ctx) error {
		limit, offset := parsePagination(c)
		if limit != 20 {
			t.Errorf("expected limit 20, got %d", limit)
		}
		if offset != 0 {
			t.Errorf("expected offset 0, got %d", offset)
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestParsePagination_Custom(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test", func(c *fiber.Ctx) error {
		limit, offset := parsePagination(c)
		if limit != 10 {
			t.Errorf("expected limit 10, got %d", limit)
		}
		if offset != 5 {
			t.Errorf("expected offset 5, got %d", offset)
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test?limit=10&offset=5", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestParsePagination_MaxLimit(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test", func(c *fiber.Ctx) error {
		limit, _ := parsePagination(c)
		if limit > 100 {
			t.Errorf("expected limit capped at 100, got %d", limit)
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test?limit=999", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestParsePagination_MinLimit(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test", func(c *fiber.Ctx) error {
		limit, _ := parsePagination(c)
		if limit < 1 {
			t.Errorf("expected limit min 1, got %d", limit)
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/test?limit=-5", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRespondSuccess(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test", func(c *fiber.Ctx) error {
		return respondSuccess(c, fiber.Map{"key": "value"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["success"] != true {
		t.Error("expected success true")
	}
	data := body["data"].(map[string]interface{})
	if data["key"] != "value" {
		t.Errorf("expected data.key='value', got '%v'", data["key"])
	}
}

func TestRespondError(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test", func(c *fiber.Ctx) error {
		return respondError(c, 404, "NOT_FOUND", "resource not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["success"] != false {
		t.Error("expected success false")
	}
	errObj := body["error"].(map[string]interface{})
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("expected error.code='NOT_FOUND', got '%v'", errObj["code"])
	}
}

func TestRespondPaginated(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/test", func(c *fiber.Ctx) error {
		return respondPaginated(c, []string{"a", "b"}, 100, 20, 0)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["success"] != true {
		t.Error("expected success true")
	}
	meta := body["meta"].(map[string]interface{})
	if meta["total"].(float64) != 100 {
		t.Errorf("expected meta.total 100, got %v", meta["total"])
	}
	if meta["limit"].(float64) != 20 {
		t.Errorf("expected meta.limit 20, got %v", meta["limit"])
	}
}
