package api

import (
	"github.com/gofiber/fiber/v2"
)

type Meta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	ErrInvalidRequest = "INVALID_REQUEST"
	ErrUnauthorized   = "UNAUTHORIZED"
	ErrTokenExpired   = "TOKEN_EXPIRED"
	ErrInvalidToken   = "INVALID_TOKEN"
	ErrForbidden      = "FORBIDDEN"
	ErrNotFound       = "NOT_FOUND"
	ErrConflict       = "CONFLICT"
	ErrValidation     = "VALIDATION_ERROR"
	ErrRateLimited    = "RATE_LIMITED"
	ErrInternal       = "INTERNAL_ERROR"
)

func respondSuccess(c *fiber.Ctx, data interface{}) error {
	return c.JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

func respondPaginated(c *fiber.Ctx, data interface{}, total, limit, offset int) error {
	return c.JSON(fiber.Map{
		"success": true,
		"data":    data,
		"meta": Meta{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	})
}

func respondError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"error": ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}

func parsePagination(c *fiber.Ctx) (limit, offset int) {
	limit = c.QueryInt("limit", 20)
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset = c.QueryInt("offset", 0)
	if offset < 0 {
		offset = 0
	}
	return
}
