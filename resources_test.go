package commentable

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest/filter"
	rbac "github.com/nicolasbonnici/gorest-rbac"
)

func TestCommentResource_validateCommentableFilter(t *testing.T) {
	config := &Config{
		AllowedTypes: []string{"post", "article", "product"},
	}

	resource := &CommentResource{
		Config: config,
	}

	tests := []struct {
		name        string
		filterField string
		values      []string
		expectError bool
	}{
		{
			name:        "valid single commentable type",
			filterField: "commentable",
			values:      []string{"post"},
			expectError: false,
		},
		{
			name:        "valid multiple commentable types",
			filterField: "commentable",
			values:      []string{"post", "article"},
			expectError: false,
		},
		{
			name:        "invalid commentable type",
			filterField: "commentable",
			values:      []string{"invalid"},
			expectError: true,
		},
		{
			name:        "mixed valid and invalid types",
			filterField: "commentable",
			values:      []string{"post", "invalid"},
			expectError: true,
		},
		{
			name:        "non-commentable field",
			filterField: "user_id",
			values:      []string{"some-uuid"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := &filter.FilterSet{
				Filters: []filter.Filter{
					{
						Field:    tt.filterField,
						Operator: filter.OpIn,
						Values:   tt.values,
					},
				},
				AllowedFields: map[string]bool{
					"commentable": true,
					"user_id":     true,
				},
			}

			filters.AllowedFields = map[string]bool{
				tt.filterField: true,
			}

			err := resource.validateCommentableFilter(filters)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

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

func TestCommentResource_isModerator(t *testing.T) {
	resource := &CommentResource{Voter: newTestVoter(t)}

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
				got = resource.isModerator(c)
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

func TestCommentResource_validateFilterLimits(t *testing.T) {
	resource := &CommentResource{}

	tests := []struct {
		name        string
		valueCount  int
		expectError bool
	}{
		{
			name:        "within limit - 1 value",
			valueCount:  1,
			expectError: false,
		},
		{
			name:        "within limit - 50 values",
			valueCount:  50,
			expectError: false,
		},
		{
			name:        "exceeds limit - 51 values",
			valueCount:  51,
			expectError: true,
		},
		{
			name:        "exceeds limit - 100 values",
			valueCount:  100,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make([]string, tt.valueCount)
			for i := 0; i < tt.valueCount; i++ {
				values[i] = "value"
			}

			filters := &filter.FilterSet{
				Filters: []filter.Filter{
					{
						Field:    "commentable",
						Operator: filter.OpIn,
						Values:   values,
					},
				},
				AllowedFields: map[string]bool{
					"commentable": true,
				},
			}

			err := resource.validateFilterLimits(filters)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
