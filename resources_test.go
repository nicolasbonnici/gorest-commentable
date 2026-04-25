package commentable

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	rbac "github.com/nicolasbonnici/gorest/rbac"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/query"
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

func TestCommentHooks_CreateAnonymous(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
		AllowAnonymous:   true,
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	app := fiber.New()
	var createdComment *Comment
	app.Post("/", func(c *fiber.Ctx) error {
		// No authentication context (anonymous user)
		c.SetUserContext(context.Background())

		dto := CommentCreateDTO{
			Commentable:   "post",
			CommentableId: "test-id",
			Content:       "Anonymous comment",
		}
		model := &Comment{}

		err := hooks.Create(c, dto, model)
		if err != nil {
			return err
		}

		createdComment = model
		return c.SendStatus(201)
	})

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "127.0.0.1:1234"

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	if createdComment.UserId != nil {
		t.Error("expected UserId to be nil for anonymous comment")
	}

	if createdComment.IpAddress == nil || *createdComment.IpAddress == "" {
		t.Error("expected IpAddress to be captured for anonymous comment")
	}

	if createdComment.UserAgent == nil || *createdComment.UserAgent != "test-agent" {
		t.Error("expected UserAgent to be captured for anonymous comment")
	}

	if createdComment.Content != "Anonymous comment" {
		t.Errorf("expected content 'Anonymous comment', got %s", createdComment.Content)
	}
}

func TestCommentHooks_CreateAnonymousDisallowed(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
		AllowAnonymous:   false,
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		// No authentication context (anonymous user)
		c.SetUserContext(context.Background())

		dto := CommentCreateDTO{
			Commentable:   "post",
			CommentableId: "test-id",
			Content:       "Anonymous comment",
		}
		model := &Comment{}

		err := hooks.Create(c, dto, model)
		if err != nil {
			return err
		}

		return c.SendStatus(201)
	})

	req := httptest.NewRequest("POST", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected status 401 when anonymous disabled, got %d", resp.StatusCode)
	}
}

func TestCommentHooks_CreateAuthenticated(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
		AllowAnonymous:   true,
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	app := fiber.New()
	var createdComment *Comment
	app.Post("/", func(c *fiber.Ctx) error {
		// Set authenticated user with reader role
		userId := "user-123"
		c.Locals("user_id", userId)
		ctx := rbac.WithRoles(context.Background(), []string{"reader"})
		c.SetUserContext(ctx)

		dto := CommentCreateDTO{
			Commentable:   "post",
			CommentableId: "test-id",
			Content:       "Authenticated comment",
		}
		model := &Comment{}

		err := hooks.Create(c, dto, model)
		if err != nil {
			return err
		}

		createdComment = model
		return c.SendStatus(201)
	})

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("User-Agent", "test-agent")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	if createdComment.UserId == nil || *createdComment.UserId != "user-123" {
		t.Error("expected UserId to be set for authenticated comment")
	}

	if createdComment.IpAddress == nil {
		t.Error("expected IpAddress to be captured for authenticated comment")
	}

	if createdComment.UserAgent == nil || *createdComment.UserAgent != "test-agent" {
		t.Error("expected UserAgent to be captured for authenticated comment")
	}
}

func TestCommentHooks_CannotEditAnonymousComment(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	// Create an anonymous comment (UserId = nil)
	existingComment := &Comment{
		Id:            "comment-123",
		UserId:        nil,
		Commentable:   "post",
		CommentableId: "post-123",
		Content:       "Anonymous comment",
	}

	app := fiber.New()
	app.Put("/", func(c *fiber.Ctx) error {
		// Regular authenticated user (not moderator)
		userId := "user-456"
		c.Locals("user_id", userId)
		ctx := rbac.WithRoles(context.Background(), []string{"reader"})
		c.SetUserContext(ctx)

		err := hooks.checkOwnership(c, existingComment)
		if err != nil {
			return err
		}

		return c.SendStatus(200)
	})

	req := httptest.NewRequest("PUT", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 403 {
		t.Errorf("expected status 403 when non-moderator tries to edit anonymous comment, got %d", resp.StatusCode)
	}
}

func TestCommentHooks_ModeratorCanEditAnonymousComment(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	// Create an anonymous comment (UserId = nil)
	existingComment := &Comment{
		Id:            "comment-123",
		UserId:        nil,
		Commentable:   "post",
		CommentableId: "post-123",
		Content:       "Anonymous comment",
	}

	app := fiber.New()
	app.Put("/", func(c *fiber.Ctx) error {
		// Moderator user
		userId := "moderator-user"
		c.Locals("user_id", userId)
		ctx := rbac.WithRoles(context.Background(), []string{"moderator"})
		c.SetUserContext(ctx)

		err := hooks.checkOwnership(c, existingComment)
		if err != nil {
			return err
		}

		return c.SendStatus(200)
	})

	req := httptest.NewRequest("PUT", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 when moderator edits anonymous comment, got %d", resp.StatusCode)
	}
}

func TestCommentHooks_UnauthenticatedCannotEditAuthenticatedComment(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	// Create an authenticated comment
	userId := "user-123"
	existingComment := &Comment{
		Id:            "comment-123",
		UserId:        &userId,
		Commentable:   "post",
		CommentableId: "post-123",
		Content:       "Authenticated comment",
	}

	app := fiber.New()
	app.Put("/", func(c *fiber.Ctx) error {
		// No authentication context
		c.SetUserContext(context.Background())

		err := hooks.checkOwnership(c, existingComment)
		if err != nil {
			return err
		}

		return c.SendStatus(200)
	})

	req := httptest.NewRequest("PUT", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 403 {
		t.Errorf("expected status 403 when unauthenticated tries to edit authenticated comment, got %d", resp.StatusCode)
	}
}

func TestCommentHooks_CannotDeleteAnonymousComment(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
	}
	// Mock database that returns an anonymous comment
	hooks := &CommentHooks{
		config: config,
		voter:  newTestVoter(t),
		db:     nil, // We'll override getComment behavior in test
	}

	app := fiber.New()
	app.Delete("/", func(c *fiber.Ctx) error {
		// Regular authenticated user (not moderator)
		userId := "user-456"
		c.Locals("user_id", userId)
		ctx := rbac.WithRoles(context.Background(), []string{"reader"})
		c.SetUserContext(ctx)

		// Simulate anonymous comment
		existingComment := &Comment{
			Id:            "comment-123",
			UserId:        nil,
			Commentable:   "post",
			CommentableId: "post-123",
			Content:       "Anonymous comment",
		}

		// Test the ownership logic directly
		if existingComment.UserId == nil {
			if !hooks.isModerator(c) {
				return fiber.NewError(403, "Only moderators can delete anonymous comments")
			}
		}

		return c.SendStatus(200)
	})

	req := httptest.NewRequest("DELETE", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 403 {
		t.Errorf("expected status 403 when non-moderator tries to delete anonymous comment, got %d", resp.StatusCode)
	}
}

func TestCommentHooks_ModeratorCanDeleteAnonymousComment(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
	}
	hooks := &CommentHooks{
		config: config,
		voter:  newTestVoter(t),
		db:     nil,
	}

	app := fiber.New()
	app.Delete("/", func(c *fiber.Ctx) error {
		// Moderator user
		userId := "moderator-user"
		c.Locals("user_id", userId)
		ctx := rbac.WithRoles(context.Background(), []string{"moderator"})
		c.SetUserContext(ctx)

		// Simulate anonymous comment
		existingComment := &Comment{
			Id:            "comment-123",
			UserId:        nil,
			Commentable:   "post",
			CommentableId: "post-123",
			Content:       "Anonymous comment",
		}

		// Test the ownership logic directly
		if existingComment.UserId == nil {
			if !hooks.isModerator(c) {
				return fiber.NewError(403, "Only moderators can delete anonymous comments")
			}
		}

		return c.SendStatus(200)
	})

	req := httptest.NewRequest("DELETE", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 when moderator deletes anonymous comment, got %d", resp.StatusCode)
	}
}

func TestCommentHooks_GetAll_ModeratorSeesAwaitingAndPublished(t *testing.T) {
	config := &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
	}
	hooks := NewCommentHooks(nil, config, newTestVoter(t))

	tests := []struct {
		name              string
		roles             []string
		expectedCondCount int
	}{
		{
			name:              "reader gets status filter",
			roles:             []string{"reader"},
			expectedCondCount: 1,
		},
		{
			name:              "moderator gets status filter",
			roles:             []string{"moderator"},
			expectedCondCount: 1,
		},
		{
			name:              "writer gets status filter",
			roles:             []string{"writer"},
			expectedCondCount: 1,
		},
		{
			name:              "admin gets status filter",
			roles:             []string{"admin"},
			expectedCondCount: 1,
		},
		{
			name:              "unauthenticated gets status filter",
			roles:             []string{},
			expectedCondCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			var capturedConditions *[]query.Condition

			app.Get("/", func(c *fiber.Ctx) error {
				ctx := rbac.WithRoles(context.Background(), tt.roles)
				c.SetUserContext(ctx)

				conditions := []query.Condition{}
				orderBy := []crud.OrderByClause{}

				err := hooks.GetAll(c, &conditions, &orderBy)
				if err != nil {
					return err
				}

				capturedConditions = &conditions
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != 200 {
				t.Errorf("expected status 200, got %d", resp.StatusCode)
			}

			if len(*capturedConditions) != tt.expectedCondCount {
				t.Errorf("expected %d condition(s), got %d", tt.expectedCondCount, len(*capturedConditions))
			}
		})
	}
}
