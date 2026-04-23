package mockserver

import (
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
	nextFlowID    int
	nextAccountID int
	nextUserID    int
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
			{"accountId": "acc-1", "service": "appmixer:slack", "displayName": "Seed Account"},
		},
		Users: []map[string]any{
			{"userId": "user-1", "email": "seed@test.com", "scope": []any{"user"}},
		},
		nextFlowID:    1000,
		nextAccountID: 1000,
		nextUserID:    1000,
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
}

// Start binds to a random port, starts the Fiber app in a goroutine, and
// returns the base URL and a stop function. The listener is bound before
// Start returns, so the address is immediately usable.
func Start() (addr string, stop func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	s := newStore()
	registerRoutes(app, s)
	go app.Listener(ln) //nolint:errcheck
	return "http://" + ln.Addr().String(), func() { _ = app.Shutdown() }
}
