package api

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/gofiber/fiber/v2"
)

var validPromptName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type promptItem struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type promptContent struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type updatePromptRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleGetPrompts(c *fiber.Ctx) error {
	names := s.Prompts.List()
	items := make([]promptItem, 0, len(names))
	for _, name := range names {
		path := filepath.Join(s.Prompts.Dir(), name+".txt")
		info, err := os.Stat(path)
		size := int64(0)
		if err == nil {
			size = info.Size()
		}
		items = append(items, promptItem{Name: name, Size: size})
	}
	return respondSuccess(c, items)
}

func (s *Server) handleGetPrompt(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" || !validPromptName.MatchString(name) {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid prompt name")
	}
	content := s.Prompts.Get(name)
	if content == "" {
		return respondError(c, fiber.StatusNotFound, ErrNotFound, "prompt not found")
	}
	return respondSuccess(c, promptContent{Name: name, Content: content})
}

func (s *Server) handleUpdatePrompt(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" || !validPromptName.MatchString(name) {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid prompt name")
	}

	var req updatePromptRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid request body")
	}
	if req.Content == "" {
		return respondError(c, fiber.StatusUnprocessableEntity, ErrValidation, "content is required")
	}

	if err := s.Prompts.Update(name, req.Content); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to save prompt")
	}

	return respondSuccess(c, promptContent{Name: name, Content: req.Content})
}

func (s *Server) handleDeletePrompt(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" || !validPromptName.MatchString(name) {
		return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid prompt name")
	}

	if err := s.Prompts.Delete(name); err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to delete prompt")
	}

	return respondSuccess(c, fiber.Map{"deleted": name})
}
