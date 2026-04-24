package mockserver

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func registerAccountsRoutes(r fiber.Router, s *Store) {
	r.Get("/accounts", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return c.JSON(s.Accounts)
	})

	r.Get("/accounts/:accountId", func(c *fiber.Ctx) error {
		accountID := c.Params("accountId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, a := range s.Accounts {
			if a["accountId"] == accountID {
				return c.JSON(a)
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Post("/accounts", func(c *fiber.Ctx) error {
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		// Upsert on (service, displayName): if a matching row exists, update its
		// mutable fields (token, profileInfo) in place and return it, keeping the
		// existing accountId.
		svc, _ := body["service"].(string)
		dn, _ := body["displayName"].(string)
		if svc != "" && dn != "" {
			for i, a := range s.Accounts {
				if a["service"] == svc && a["displayName"] == dn {
					if tok, ok := body["token"]; ok {
						s.Accounts[i]["token"] = tok
					}
					if pi, ok := body["profileInfo"]; ok {
						s.Accounts[i]["profileInfo"] = pi
					}
					return c.JSON(s.Accounts[i])
				}
			}
		}
		body["accountId"] = fmt.Sprintf("acc-%d", s.nextAccountID)
		s.nextAccountID++
		s.Accounts = append(s.Accounts, body)
		return c.JSON(body)
	})

	r.Put("/accounts/:accountId", func(c *fiber.Ctx) error {
		accountID := c.Params("accountId")
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, a := range s.Accounts {
			if a["accountId"] == accountID {
				if dn, ok := body["displayName"]; ok {
					s.Accounts[i]["displayName"] = dn
				}
				return c.JSON(s.Accounts[i])
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Delete("/accounts/:accountId", func(c *fiber.Ctx) error {
		accountID := c.Params("accountId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, a := range s.Accounts {
			if a["accountId"] == accountID {
				s.Accounts = append(s.Accounts[:i], s.Accounts[i+1:]...)
				return c.JSON(fiber.Map{})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})

	r.Post("/accounts/:accountId/test", func(c *fiber.Ctx) error {
		accountID := c.Params("accountId")
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, a := range s.Accounts {
			if a["accountId"] == accountID {
				return c.JSON(fiber.Map{"revoked": false})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	})
}
