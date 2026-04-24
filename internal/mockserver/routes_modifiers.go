package mockserver

import "github.com/gofiber/fiber/v2"

func registerModifiersRoutes(r fiber.Router, s *Store) {
	r.Get("/modifiers", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.Modifiers == nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.JSON(s.Modifiers)
	})

	r.Put("/modifiers", func(c *fiber.Ctx) error {
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.Modifiers = body
		return c.JSON(body)
	})

	r.Delete("/modifiers", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.Modifiers = nil
		return c.JSON(fiber.Map{})
	})
}
