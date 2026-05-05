package mockserver

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func registerFlowsRoutes(r fiber.Router, s *Store) {
	r.Get("/flows", func(c *fiber.Ctx) error {
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		limit, _ := strconv.Atoi(c.Query("limit", "100"))

		s.mu.Lock()
		defer s.mu.Unlock()

		flows := s.Flows
		if offset > len(flows) {
			offset = len(flows)
		}
		end := offset + limit
		if end > len(flows) {
			end = len(flows)
		}
		result := flows[offset:end]
		if result == nil {
			result = []map[string]any{}
		}
		return c.JSON(result)
	})

	r.Get("/flows/:flowId", func(c *fiber.Ctx) error {
		flowID := c.Params("flowId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, f := range s.Flows {
			if f["flowId"] == flowID {
				return c.JSON(f)
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Post("/flows", func(c *fiber.Ctx) error {
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		flowID := fmt.Sprintf("flow-%d", s.nextFlowID)
		s.nextFlowID++
		body["flowId"] = flowID
		body["stage"] = "stopped"
		s.Flows = append(s.Flows, body)
		return c.JSON(fiber.Map{"flowId": flowID})
	})

	r.Put("/flows/:flowId", func(c *fiber.Ctx) error {
		flowID := c.Params("flowId")
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		// Reject attempts to set server-managed fields, matching real API behaviour.
		if _, hasStage := body["stage"]; hasStage {
			return c.Status(400).JSON(fiber.Map{"error": "stage is read-only"})
		}
		for i, f := range s.Flows {
			if f["flowId"] == flowID {
				// Restore server-managed fields the client must not set.
				body["flowId"] = flowID
				if stage, ok := f["stage"]; ok {
					body["stage"] = stage
				}
				s.Flows[i] = body
				return c.JSON(body)
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Delete("/flows/:flowId", func(c *fiber.Ctx) error {
		flowID := c.Params("flowId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, f := range s.Flows {
			if f["flowId"] == flowID {
				s.Flows = append(s.Flows[:i], s.Flows[i+1:]...)
				return c.JSON(fiber.Map{})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})
}
