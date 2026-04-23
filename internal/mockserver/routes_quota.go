package mockserver

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
)

// The Appmixer Quota API does not expose a per-name GET — only a list of all
// quotas. Reads filter client-side.

func registerQuotaRoutes(r fiber.Router, s *Store) {
	r.Get("/quota", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := make([]map[string]any, 0, len(s.Quotas))
		for _, q := range s.Quotas {
			out = append(out, q)
		}
		return c.JSON(out)
	})

	r.Put("/quota/:name", func(c *fiber.Ctx) error {
		name := c.Params("name")
		var body struct {
			Source string `json:"source"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		existing, ok := s.Quotas[name]
		if !ok {
			existing = map[string]any{
				"id":            newQuotaID(),
				"name":          name,
				"defaultSource": "",
			}
		}
		existing["source"] = body.Source
		existing["isCustom"] = true
		s.Quotas[name] = existing
		return c.JSON(existing)
	})

	r.Delete("/quota/:name", func(c *fiber.Ctx) error {
		name := c.Params("name")
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.Quotas[name]; !ok {
			return c.Status(404).JSON(fiber.Map{"acknowledged": true, "deletedCount": 0})
		}
		delete(s.Quotas, name)
		return c.JSON(fiber.Map{"acknowledged": true, "deletedCount": 1})
	})
}

func newQuotaID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
