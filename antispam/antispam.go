// Package antispam provides self-hosted, dependency-free anti-automation
// defenses for anonymous comment creation: a stateless hashcash-style
// proof-of-work challenge, a honeypot/minimum-fill-time check, and layered
// fixed-window rate limiting (per IP, per target, and global).
//
// Every layer is disabled unless explicitly enabled through Config, so existing
// deployments are unaffected until they opt in. None of the layers rely on the
// User-Agent header, and the caller is expected to supply a trustworthy client
// IP (for example fiber.Ctx.IP() with the server's TrustProxy configured) so
// that a spoofed X-Forwarded-For cannot defeat the per-IP limit.
package antispam

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// Sentinel errors returned by CheckSubmission. They are intentionally coarse so
// callers can map them to a single generic client message without leaking which
// layer rejected the request.
var (
	ErrHoneypot         = errors.New("antispam: honeypot field populated")
	ErrRateLimited      = errors.New("antispam: rate limit exceeded")
	ErrChallengeFormat  = errors.New("antispam: malformed challenge")
	ErrBadSignature     = errors.New("antispam: invalid challenge signature")
	ErrExpired          = errors.New("antispam: challenge expired")
	ErrTooFast          = errors.New("antispam: submitted before minimum fill time")
	ErrInsufficientWork = errors.New("antispam: proof of work does not meet difficulty")
)

// Config controls the anti-spam layers. The zero value is fully disabled.
type Config struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Secret signs proof-of-work challenges. When empty a random per-process
	// secret is generated, which means challenges do not survive a restart and
	// are not shared across instances; set it explicitly for multi-instance
	// deployments.
	Secret string `json:"secret" yaml:"secret"`

	// PoWBits is the number of leading zero bits required in the solution hash.
	// Each extra bit doubles the expected client work. 0 disables proof of work.
	PoWBits int `json:"pow_bits" yaml:"pow_bits"`

	// ChallengeTTLSecs is how long a challenge remains valid after issuance.
	ChallengeTTLSecs int `json:"challenge_ttl_secs" yaml:"challenge_ttl_secs"`

	// MinFillSecs is the minimum delay between a challenge being issued and the
	// comment being submitted. 0 disables the fill-time check.
	MinFillSecs int `json:"min_fill_secs" yaml:"min_fill_secs"`

	// Honeypot enables the hidden-field check (the field must arrive empty).
	Honeypot bool `json:"honeypot" yaml:"honeypot"`

	// Rate limits. A value <= 0 disables that axis. The window is shared.
	RatePerIP      int `json:"rate_per_ip" yaml:"rate_per_ip"`
	RatePerTarget  int `json:"rate_per_target" yaml:"rate_per_target"`
	RateGlobal     int `json:"rate_global" yaml:"rate_global"`
	RateWindowSecs int `json:"rate_window_secs" yaml:"rate_window_secs"`
}

// Service applies the configured anti-spam layers.
type Service struct {
	cfg     Config
	secret  []byte
	limiter *windowLimiter
}

// New builds a Service from cfg, generating a random signing secret when none is
// configured.
func New(cfg Config) *Service {
	secret := []byte(cfg.Secret)
	if len(secret) == 0 {
		secret = randomBytes(32)
	}
	window := time.Duration(cfg.RateWindowSecs) * time.Second
	if window <= 0 {
		window = time.Minute
	}
	return &Service{
		cfg:     cfg,
		secret:  secret,
		limiter: newWindowLimiter(window),
	}
}

// Enabled reports whether anti-spam enforcement is active.
func (s *Service) Enabled() bool { return s.cfg.Enabled }

// PoWBits exposes the configured difficulty so the challenge endpoint can tell
// clients how much work to perform.
func (s *Service) PoWBits() int { return s.cfg.PoWBits }

// ChallengeTTL exposes the configured validity window.
func (s *Service) ChallengeTTL() time.Duration {
	return time.Duration(s.cfg.ChallengeTTLSecs) * time.Second
}

// IssueChallenge mints a fresh signed challenge token.
func (s *Service) IssueChallenge(now time.Time) string {
	return issueChallenge(s.secret, s.cfg.PoWBits, now)
}

// Submission bundles the anti-spam inputs carried by a create request.
type Submission struct {
	IP        string
	Target    string // commentable type + id, scoping the per-target limit
	Challenge string
	Solution  string
	Honeypot  string
}

// CheckSubmission runs the enabled layers in increasing order of cost and
// returns the first failure, or nil when the submission is accepted.
func (s *Service) CheckSubmission(sub Submission, now time.Time) error {
	if !s.cfg.Enabled {
		return nil
	}

	if s.cfg.Honeypot && sub.Honeypot != "" {
		return ErrHoneypot
	}

	if err := s.checkRate(sub); err != nil {
		return err
	}

	if s.cfg.PoWBits > 0 || s.cfg.MinFillSecs > 0 {
		ttl := s.ChallengeTTL()
		minFill := time.Duration(s.cfg.MinFillSecs) * time.Second
		if err := verifyChallenge(s.secret, sub.Challenge, sub.Solution, now, ttl, minFill); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) checkRate(sub Submission) error {
	if s.cfg.RateGlobal > 0 && !s.limiter.allow("global", s.cfg.RateGlobal) {
		return ErrRateLimited
	}
	if s.cfg.RatePerIP > 0 && sub.IP != "" && !s.limiter.allow("ip:"+sub.IP, s.cfg.RatePerIP) {
		return ErrRateLimited
	}
	if s.cfg.RatePerTarget > 0 && sub.Target != "" && !s.limiter.allow("target:"+sub.Target, s.cfg.RatePerTarget) {
		return ErrRateLimited
	}
	return nil
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	// crypto/rand.Read is documented to always succeed on supported platforms
	// (it never returns an error since Go 1.24), so the error is not actionable.
	_, _ = rand.Read(b)
	return b
}

func randomHex(n int) string {
	return hex.EncodeToString(randomBytes(n))
}
