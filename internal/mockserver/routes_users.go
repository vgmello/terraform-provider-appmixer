package mockserver

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func registerUsersRoutes(r fiber.Router, s *Store) {
	r.Post("/user", func(c *fiber.Ctx) error {
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		userID := fmt.Sprintf("user-%d", s.nextUserID)
		s.nextUserID++
		body["userId"] = userID
		s.Users = append(s.Users, body)
		return c.JSON(fiber.Map{
			"token": "mock-token",
			"user":  fiber.Map{"id": userID},
		})
	})

	r.Get("/users", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return c.JSON(s.Users)
	})

	r.Get("/users/:userId", func(c *fiber.Ctx) error {
		userID := c.Params("userId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, u := range s.Users {
			if u["userId"] == userID {
				return c.JSON(u)
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Put("/users/:userId", func(c *fiber.Ctx) error {
		userID := c.Params("userId")
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, u := range s.Users {
			if u["userId"] == userID {
				// Merge update fields into the existing user. The update request
				// uses "email" instead of "username", so we preserve the existing
				// username and other fields not present in the update body.
				for k, v := range body {
					u[k] = v
				}
				u["userId"] = userID
				s.Users[i] = u
				return c.JSON(u)
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Delete("/users/:userId", func(c *fiber.Ctx) error {
		userID := c.Params("userId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, u := range s.Users {
			if u["userId"] == userID {
				s.Users = append(s.Users[:i], s.Users[i+1:]...)
				return c.JSON(fiber.Map{"ticket": "delete-ticket-" + userID})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Get("/users/:userId/delete-status/:ticket", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "completed"})
	})

	// Admin password reset by email. The real API returns 200 on success and
	// 404 if no user with that email exists.
	r.Post("/user/reset-password", func(c *fiber.Ctx) error {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		if body.Email == "" || body.Password == "" {
			return c.Status(400).JSON(fiber.Map{"error": "email and password are required"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, u := range s.Users {
			if u["username"] == body.Email || u["email"] == body.Email {
				u["password"] = body.Password
				return c.JSON(fiber.Map{"ok": true})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	})
}
