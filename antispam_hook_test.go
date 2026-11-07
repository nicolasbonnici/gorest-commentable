package commentable

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/nicolasbonnici/gorest-commentable/antispam"
)

// runCreate drives hooks.Create for an anonymous caller and returns the fiber
// error (if any) produced by the hook.
func runCreate(t *testing.T, hooks *CommentHooks, dto CommentCreateDTO, remoteAddr string) error {
	t.Helper()
	app := fiber.New()
	var hookErr error
	app.Post("/", func(c fiber.Ctx) error {
		c.SetContext(context.Background())
		model := &Comment{}
		hookErr = hooks.Create(c, dto, model)
		return c.SendStatus(201)
	})
	req := httptest.NewRequest("POST", "/", nil)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return hookErr
}

func statusOf(err error) int {
	if fe, ok := err.(*fiber.Error); ok {
		return fe.Code
	}
	return 0
}

func antispamConfig(as antispam.Config) *Config {
	return &Config{
		AllowedTypes:     []string{"post"},
		MaxContentLength: 10000,
		AllowAnonymous:   true,
		DefaultStatus:    StatusAwaiting,
		AntiSpam:         as,
	}
}

const validTargetID = "3f1a7c22-5f6e-4a0b-9d3e-1c2b4a5d6e7f"

func TestCreate_AntiSpamDisabled_IgnoresHoneypot(t *testing.T) {
	hooks := NewCommentHooks(nil, antispamConfig(antispam.Config{Enabled: false, Honeypot: true}), newTestVoter(t))
	dto := CommentCreateDTO{
		Commentable:   "post",
		CommentableId: validTargetID,
		Content:       "hi",
		Website:       "http://spam.example", // honeypot populated but anti-spam off
	}
	if err := runCreate(t, hooks, dto, "127.0.0.1:1111"); err != nil {
		t.Fatalf("disabled anti-spam should ignore honeypot, got %v", err)
	}
}

func TestCreate_Honeypot(t *testing.T) {
	hooks := NewCommentHooks(nil, antispamConfig(antispam.Config{Enabled: true, Honeypot: true}), newTestVoter(t))

	spam := CommentCreateDTO{
		Commentable:   "post",
		CommentableId: validTargetID,
		Content:       "buy stuff",
		Website:       "http://spam.example",
	}
	if err := runCreate(t, hooks, spam, "127.0.0.1:1111"); statusOf(err) != 400 {
		t.Fatalf("populated honeypot should be rejected with 400, got %v", err)
	}

	clean := spam
	clean.Website = ""
	if err := runCreate(t, hooks, clean, "127.0.0.1:1111"); err != nil {
		t.Fatalf("empty honeypot should pass, got %v", err)
	}
}

func TestCreate_RateLimitPerIP(t *testing.T) {
	hooks := NewCommentHooks(nil, antispamConfig(antispam.Config{
		Enabled:        true,
		RatePerIP:      1,
		RateWindowSecs: 60,
	}), newTestVoter(t))

	dto := CommentCreateDTO{
		Commentable:   "post",
		CommentableId: validTargetID,
		Content:       "hello",
	}

	// The in-memory test client presents a single client IP, so repeated
	// submissions share the per-IP bucket: the second is rejected once the limit
	// of 1 is reached. Per-IP isolation across distinct addresses is covered by
	// the antispam package's TestCheckSubmission_RateLimits.
	if err := runCreate(t, hooks, dto, "203.0.113.5:4444"); err != nil {
		t.Fatalf("first submission should pass, got %v", err)
	}
	if err := runCreate(t, hooks, dto, "203.0.113.5:4444"); statusOf(err) != fiber.StatusTooManyRequests {
		t.Fatalf("second submission should be 429, got %v", err)
	}
}
