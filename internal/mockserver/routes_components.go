package mockserver

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func registerComponentsRoutes(r fiber.Router, s *Store) {
	// POST /components — publish a component from a zip upload.
	// The mock reads the zip to extract the component selector from the
	// directory structure (segments joined with dots), generates a ticket
	// that is immediately marked as finished, and registers the component
	// so subsequent reads can find it.
	r.Post("/components", func(c *fiber.Ctx) error {
		body := c.Body()

		s.mu.Lock()
		defer s.mu.Unlock()

		ticketID := fmt.Sprintf("ticket-%d", s.nextTicketID)
		s.nextTicketID++

		// Try to extract the component selector from the zip contents.
		selector := selectorFromZip(body)

		if selector != "" {
			s.Components[selector] = map[string]any{
				"selector": selector,
			}
		}

		s.Tickets[ticketID] = map[string]any{
			"finished": time.Now().UTC().Format(time.RFC3339),
		}

		return c.JSON(fiber.Map{"ticket": ticketID})
	})

	// GET /components/uploader/:ticket — poll a publish ticket.
	r.Get("/components/uploader/:ticket", func(c *fiber.Ctx) error {
		ticketID := c.Params("ticket")
		s.mu.Lock()
		defer s.mu.Unlock()
		t, ok := s.Tickets[ticketID]
		if !ok {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.JSON(t)
	})

	// GET /apps/components — list components for a given app (selector).
	r.Get("/apps/components", func(c *fiber.Ctx) error {
		app := c.Query("app")
		s.mu.Lock()
		defer s.mu.Unlock()

		var result []map[string]any
		for sel, comp := range s.Components {
			if app == "" || sel == app || strings.HasPrefix(sel, app+".") {
				result = append(result, comp)
			}
		}
		if result == nil {
			result = []map[string]any{}
		}
		return c.JSON(result)
	})

	// DELETE /components/* — remove a published component.
	// Use a wildcard because Fiber truncates :params at dots.
	r.Delete("/components/*", func(c *fiber.Ctx) error {
		selector := c.Params("*")
		s.mu.Lock()
		defer s.mu.Unlock()

		// Delete exact match and any sub-components that start with selector.
		deleted := false
		for sel := range s.Components {
			if sel == selector || strings.HasPrefix(sel, selector+".") {
				delete(s.Components, sel)
				deleted = true
			}
		}
		if !deleted {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.JSON(fiber.Map{})
	})
}

// selectorFromZip attempts to read body as a zip archive and derives a
// dot-separated component selector from the first file's directory path.
// For example, a file at "appmixer/test/service.json" yields "appmixer.test".
// Returns an empty string if the body is not a valid zip or contains no files.
func selectorFromZip(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	if len(r.File) == 0 {
		return ""
	}

	// Use the first file entry to derive the selector.
	name := r.File[0].Name
	parts := strings.Split(name, "/")

	var segments []string
	for _, p := range parts {
		if p != "" {
			segments = append(segments, p)
		}
	}
	// Need at least one directory segment plus a file name; a lone root-level
	// file has no selector to derive.
	if len(segments) <= 1 {
		return ""
	}
	segments = segments[:len(segments)-1]
	return strings.Join(segments, ".")
}
