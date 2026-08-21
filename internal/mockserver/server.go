package mockserver

import (
	"log"
	"net"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// Store holds all in-memory state for the mock server. All handlers lock mu
// for the duration of their store access.
type Store struct {
	mu            sync.Mutex
	Config        []map[string]any
	ServiceConfig []map[string]any
	ACL           map[string][]any
	Modifiers     map[string]any
	Flows         []map[string]any
	Accounts      []map[string]any
	Users         []map[string]any
	Quotas        map[string]map[string]any
	Components    map[string]map[string]any // keyed by selector
	Tickets       map[string]map[string]any // keyed by ticket ID
	nextFlowID    int
	nextAccountID int
	nextUserID    int
	nextTicketID  int
}

func newStore() *Store {
	return &Store{
		Config: []map[string]any{
			{"key": "API_URL", "value": "https://api.example.com"},
		},
		ServiceConfig: []map[string]any{
			{"serviceId": "appmixer:google", "client_id": "seed-client-id"},
		},
		ACL: map[string][]any{
			"components": {},
			"routes":     {},
		},
		Modifiers: nil,
		Flows: []map[string]any{
			{"flowId": "flow-1", "stage": "stopped"},
			{"flowId": "flow-2", "stage": "stopped"},
		},
		Accounts: []map[string]any{
			{"accountId": "acc-1", "service": "appmixer:slack", "name": "seed-account", "displayName": "Seed Account"},
		},
		Users: []map[string]any{
			{"userId": "user-1", "username": "seed@test.com", "scope": []any{"user"}},
		},
		Quotas: map[string]map[string]any{
			"appmixer:seed": {
				"id":            "seed-quota-id",
				"name":          "appmixer:seed",
				"defaultSource": "'use strict';\nmodule.exports = { rules: [] };\n",
				"isCustom":      nil,
				"source":        "'use strict';\nmodule.exports = { rules: [] };\n",
			},
		},
		Components:    map[string]map[string]any{},
		Tickets:       map[string]map[string]any{},
		nextFlowID:    1000,
		nextAccountID: 1000,
		nextUserID:    1000,
		nextTicketID:  1000,
	}
}

func authMiddleware(c *fiber.Ctx) error {
	if c.Get("Authorization") != "Bearer mock-token" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	return c.Next()
}

func registerRoutes(app *fiber.App, s *Store) {
	registerAuthRoutes(app, s)
	api := app.Group("", authMiddleware)
	registerConfigRoutes(api, s)
	registerServiceConfigRoutes(api, s)
	registerACLRoutes(api, s)
	registerModifiersRoutes(api, s)
	registerFlowsRoutes(api, s)
	registerAccountsRoutes(api, s)
	registerUsersRoutes(api, s)
	registerQuotaRoutes(api, s)
	registerComponentsRoutes(api, s)
}

// Start binds to a random port, starts the Fiber app in a goroutine, and
// returns the base URL and a stop function. The listener is bound before
// Start returns, so the address is immediately usable.
func Start() (addr string, stop func()) {
	addr, _, stop = StartWithStore()
	return addr, stop
}

// StartWithStore is Start plus a handle on the server's in-memory state, so a
// test can simulate changes made outside Terraform (drift) without going
// through the API.
func StartWithStore() (addr string, store *Store, stop func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true, Immutable: true})
	s := newStore()
	registerRoutes(app, s)
	go func() {
		if err := app.Listener(ln); err != nil {
			// Unexpected listener error (graceful shutdown returns nil).
			log.Printf("mock server listener error: %v", err)
		}
	}()
	return "http://" + ln.Addr().String(), s, func() { _ = app.Shutdown() }
}

// MutateFlowByName applies fn to the stored flow descriptor of the first flow
// with the given name, simulating an out-of-band change. It reports whether a
// matching flow with a descriptor was found.
func (s *Store) MutateFlowByName(name string, fn func(flow map[string]any)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.Flows {
		if f["name"] != name {
			continue
		}
		flow, ok := f["flow"].(map[string]any)
		if !ok {
			return false
		}
		fn(flow)
		return true
	}
	return false
}
