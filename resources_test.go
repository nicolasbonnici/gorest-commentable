package commentable

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	rbac "github.com/nicolasbonnici/gorest-rbac"
)

func newTestVoter(t *testing.T) rbac.Voter {
	t.Helper()
	voter, err := rbac.NewVoter(rbac.Config{
		DefaultPolicy: rbac.DenyAll,
		SuperuserRole: "admin",
		RoleHierarchy: map[string][]string{
			"writer":    {"moderator"},
			"moderator": {"reader"},
		},
		StrictMode:         false,
		DefaultFieldPolicy: "deny",
	})
	if err != nil {
		t.Fatalf("failed to create voter: %v", err)
	}
	return voter
}

func TestCommentHooks_isModerator(t *testing.T) {
	config := &Config{
		AllowedTypes: []string{"post"},
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	tests := []struct {
		name     string
		roles    []string
		expected bool
	}{
		{"unauthenticated", []string{}, false},
		{"reader", []string{"reader"}, false},
		{"moderator", []string{"moderator"}, true},
		{"writer inherits moderator", []string{"writer"}, true},
		{"admin is superuser", []string{"admin"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			var got bool
			app.Get("/", func(c *fiber.Ctx) error {
				c.SetUserContext(rbac.WithRoles(context.Background(), tt.roles))
				got = hooks.isModerator(c)
				return c.SendStatus(200)
			})
			_, err := app.Test(httptest.NewRequest("GET", "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.expected {
				t.Errorf("roles %v: got %v, want %v", tt.roles, got, tt.expected)
			}
		})
	}
}

