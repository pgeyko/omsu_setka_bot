package api

import (
	"github.com/gofiber/fiber/v2"
)

func (s *Server) handleStatsTokens(c *fiber.Ctx) error {
	var totalToday int
	s.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(SUM(input_tokens + output_tokens), 0) FROM llm_requests
		 WHERE date(created_at) = date('now')`).Scan(&totalToday)

	rows, err := s.DB.QueryContext(c.Context(),
		`SELECT COALESCE(provider, ''), COALESCE(SUM(input_tokens + output_tokens), 0)
		 FROM llm_requests WHERE date(created_at) = date('now')
		 GROUP BY provider`)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to query tokens")
	}
	defer rows.Close()

	byProvider := make(map[string]int)
	for rows.Next() {
		var provider string
		var tokens int
		rows.Scan(&provider, &tokens)
		byProvider[provider] = tokens
	}

	return respondSuccess(c, fiber.Map{
		"total_today": totalToday,
		"by_provider": byProvider,
	})
}

func (s *Server) handleStatsRequests(c *fiber.Ctx) error {
	var totalToday int
	s.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(COUNT(*), 0) FROM llm_requests
		 WHERE date(created_at) = date('now')`).Scan(&totalToday)

	rows, err := s.DB.QueryContext(c.Context(),
		`SELECT type, COALESCE(COUNT(*), 0) FROM llm_requests
		 WHERE date(created_at) = date('now') GROUP BY type`)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to query requests")
	}
	defer rows.Close()

	byType := make(map[string]int)
	for rows.Next() {
		var reqType string
		var count int
		rows.Scan(&reqType, &count)
		byType[reqType] = count
	}

	return respondSuccess(c, fiber.Map{
		"total_today": totalToday,
		"by_type":     byType,
	})
}

func (s *Server) handleStatsForwards(c *fiber.Ctx) error {
	var autoForwards int
	s.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(COUNT(*), 0) FROM processed_messages
		 WHERE action = 'forwarded'`).Scan(&autoForwards)

	return respondSuccess(c, fiber.Map{
		"auto_forwards":   autoForwards,
		"manual_forwards": 0,
	})
}

func (s *Server) handleStatsMessages(c *fiber.Ctx) error {
	var total, skipped, forwarded, lowConf int
	s.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(COUNT(*), 0) FROM processed_messages`).Scan(&total)
	s.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(COUNT(*), 0) FROM processed_messages WHERE action = 'skipped'`).Scan(&skipped)
	s.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(COUNT(*), 0) FROM processed_messages WHERE action = 'forwarded'`).Scan(&forwarded)
	s.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(COUNT(*), 0) FROM processed_messages WHERE action = 'low_confidence'`).Scan(&lowConf)

	return respondSuccess(c, fiber.Map{
		"total_processed": total,
		"skipped":         skipped,
		"forwarded":       forwarded,
		"low_confidence":  lowConf,
	})
}

func (s *Server) handleStatsProviders(c *fiber.Ctx) error {
	rows, err := s.DB.QueryContext(c.Context(),
		`SELECT provider, COUNT(*) as total, SUM(input_tokens + output_tokens) as tokens,
				COALESCE(SUM(cost_usd), 0) as cost, MAX(created_at) as last_used
		 FROM llm_requests WHERE provider != ''
		 GROUP BY provider ORDER BY total DESC`)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, ErrInternal, "failed to query providers")
	}
	defer rows.Close()

	providers := make([]fiber.Map, 0)
	for rows.Next() {
		var provider string
		var total, tokens int
		var cost float64
		var lastUsed interface{}
		rows.Scan(&provider, &total, &tokens, &cost, &lastUsed)
		providers = append(providers, fiber.Map{
			"name":      provider,
			"requests":  total,
			"tokens":    tokens,
			"cost_usd":  cost,
			"last_used": lastUsed,
		})
	}

	return respondSuccess(c, providers)
}
