package mockserver

import "github.com/gofiber/fiber/v2"

func registerConfigRoutes(r fiber.Router, s *Store) {
	r.Get("/config", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return c.JSON(s.Config)
	})

	r.Post("/config", func(c *fiber.Ctx) error {
		var entry map[string]any
		if err := c.BodyParser(&entry); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.Config = append(s.Config, entry)
		return c.JSON(entry)
	})

	r.Delete("/config/:key", func(c *fiber.Ctx) error {
		key := c.Params("key")
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, e := range s.Config {
			if e["key"] == key {
				s.Config = append(s.Config[:i], s.Config[i+1:]...)
				return c.JSON(fiber.Map{"ok": true})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})
}
