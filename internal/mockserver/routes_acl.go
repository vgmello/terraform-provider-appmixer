package mockserver

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

func registerACLRoutes(r fiber.Router, s *Store) {
	r.Get("/acl/:type", func(c *fiber.Ctx) error {
		aclType := c.Params("type")
		s.mu.Lock()
		defer s.mu.Unlock()
		rules, ok := s.ACL[aclType]
		if !ok || rules == nil {
			return c.JSON([]any{})
		}
		return c.JSON(rules)
	})

	r.Post("/acl/:type", func(c *fiber.Ctx) error {
		aclType := c.Params("type")
		body := make([]byte, len(c.Body()))
		copy(body, c.Body())
		var rules []any
		if err := json.Unmarshal(body, &rules); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if rules == nil {
			rules = []any{}
		}
		s.ACL[aclType] = rules
		return c.JSON(rules)
	})
}
