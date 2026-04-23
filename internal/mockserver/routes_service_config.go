package mockserver

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func registerServiceConfigRoutes(r fiber.Router, s *Store) {
	r.Get("/service-config", func(c *fiber.Ctx) error {
		pattern := c.Query("pattern")
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		limit, _ := strconv.Atoi(c.Query("limit", "100"))

		s.mu.Lock()
		defer s.mu.Unlock()

		var result []map[string]any
		for _, e := range s.ServiceConfig {
			sid, _ := e["serviceId"].(string)
			if pattern == "" || strings.Contains(sid, pattern) {
				result = append(result, e)
			}
		}
		if result == nil {
			result = []map[string]any{}
		}
		if offset > len(result) {
			offset = len(result)
		}
		end := offset + limit
		if end > len(result) {
			end = len(result)
		}
		return c.JSON(result[offset:end])
	})

	r.Get("/service-config/:serviceId", func(c *fiber.Ctx) error {
		serviceID := c.Params("serviceId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, e := range s.ServiceConfig {
			if e["serviceId"] == serviceID {
				return c.JSON(e)
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Post("/service-config", func(c *fiber.Ctx) error {
		var entry map[string]any
		if err := c.BodyParser(&entry); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.ServiceConfig = append(s.ServiceConfig, entry)
		return c.JSON(entry)
	})

	r.Put("/service-config/:serviceId", func(c *fiber.Ctx) error {
		serviceID := c.Params("serviceId")
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, e := range s.ServiceConfig {
			if e["serviceId"] == serviceID {
				s.ServiceConfig[i] = body
				return c.JSON(body)
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Delete("/service-config/:serviceId", func(c *fiber.Ctx) error {
		serviceID := c.Params("serviceId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, e := range s.ServiceConfig {
			if e["serviceId"] == serviceID {
				s.ServiceConfig = append(s.ServiceConfig[:i], s.ServiceConfig[i+1:]...)
				return c.JSON(fiber.Map{})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})
}
