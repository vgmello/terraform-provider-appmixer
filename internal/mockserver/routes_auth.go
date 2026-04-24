package mockserver

import "github.com/gofiber/fiber/v2"

func registerAuthRoutes(r fiber.Router, s *Store) {
	r.Post("/user/auth", func(c *fiber.Ctx) error {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		if body.Username != "admin@test.com" || body.Password != "test123" {
			return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
		}
		return c.JSON(fiber.Map{"token": "mock-token"})
	})
}
