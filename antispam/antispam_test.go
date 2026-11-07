package antispam

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// solve finds a solution string whose hash meets the challenge difficulty.
func solve(t *testing.T, token string, bits int) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		t.Fatalf("bad token: %q", token)
	}
	payload := strings.Join(parts[:3], ".")
	for i := 0; i < 1<<24; i++ {
		s := strconv.Itoa(i)
		if meetsDifficulty(payload, s, bits) {
			return s
		}
	}
	t.Fatalf("no solution found for %d bits", bits)
	return ""
}

func TestProofOfWork_RoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Unix(1_000_000, 0)
	const bits = 8

	token := issueChallenge(secret, bits, now)
	solution := solve(t, token, bits)

	// Valid after the minimum fill time, before the TTL.
	at := now.Add(5 * time.Second)
	if err := verifyChallenge(secret, token, solution, at, time.Minute, 2*time.Second); err != nil {
		t.Fatalf("expected valid challenge, got %v", err)
	}
}

func TestProofOfWork_Rejections(t *testing.T) {
	secret := []byte("test-secret")
	now := time.Unix(1_000_000, 0)
	const bits = 8
	token := issueChallenge(secret, bits, now)
	solution := solve(t, token, bits)

	tests := []struct {
		name     string
		token    string
		solution string
		at       time.Time
		ttl      time.Duration
		minFill  time.Duration
		want     error
	}{
		{"malformed", "a.b.c", solution, now.Add(5 * time.Second), time.Minute, 0, ErrChallengeFormat},
		{"bad signature", token[:len(token)-1] + "0", solution, now.Add(5 * time.Second), time.Minute, 0, ErrBadSignature},
		{"wrong secret", issueChallenge([]byte("other"), bits, now), solution, now.Add(5 * time.Second), time.Minute, 0, ErrBadSignature},
		{"expired", token, solution, now.Add(2 * time.Minute), time.Minute, 0, ErrExpired},
		{"from the future", token, solution, now.Add(-10 * time.Second), time.Minute, 0, ErrExpired},
		{"too fast", token, solution, now.Add(1 * time.Second), time.Minute, 5 * time.Second, ErrTooFast},
		{"insufficient work", token, "definitely-not-a-solution", now.Add(5 * time.Second), time.Minute, 0, ErrInsufficientWork},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyChallenge(secret, tc.token, tc.solution, tc.at, tc.ttl, tc.minFill)
			if err != tc.want {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestCheckSubmission_Disabled(t *testing.T) {
	s := New(Config{Enabled: false, Honeypot: true})
	// Even a populated honeypot passes when the service is disabled.
	if err := s.CheckSubmission(Submission{Honeypot: "x"}, time.Now()); err != nil {
		t.Fatalf("disabled service should accept everything, got %v", err)
	}
}

func TestCheckSubmission_Honeypot(t *testing.T) {
	s := New(Config{Enabled: true, Honeypot: true})
	if err := s.CheckSubmission(Submission{Honeypot: "http://spam"}, time.Now()); err != ErrHoneypot {
		t.Fatalf("want ErrHoneypot, got %v", err)
	}
	if err := s.CheckSubmission(Submission{Honeypot: ""}, time.Now()); err != nil {
		t.Fatalf("empty honeypot should pass, got %v", err)
	}
}

func TestCheckSubmission_ProofOfWork(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	s := New(Config{
		Enabled:          true,
		Secret:           "fixed-secret",
		PoWBits:          8,
		ChallengeTTLSecs: 300,
	})

	token := s.IssueChallenge(now)
	solution := solve(t, token, 8)

	at := now.Add(10 * time.Second)
	if err := s.CheckSubmission(Submission{Challenge: token, Solution: solution}, at); err != nil {
		t.Fatalf("valid PoW submission should pass, got %v", err)
	}
	if err := s.CheckSubmission(Submission{Challenge: token, Solution: "nope"}, at); err != ErrInsufficientWork {
		t.Fatalf("want ErrInsufficientWork, got %v", err)
	}
	if err := s.CheckSubmission(Submission{Challenge: "", Solution: ""}, at); err != ErrChallengeFormat {
		t.Fatalf("missing challenge should be rejected, got %v", err)
	}
}

func TestCheckSubmission_RateLimits(t *testing.T) {
	s := New(Config{
		Enabled:        true,
		RatePerIP:      2,
		RatePerTarget:  3,
		RateGlobal:     100,
		RateWindowSecs: 60,
	})
	now := time.Unix(3_000_000, 0)

	// Per-IP limit of 2: third submission from the same IP is blocked.
	for i := 0; i < 2; i++ {
		if err := s.CheckSubmission(Submission{IP: "1.2.3.4", Target: "post:" + strconv.Itoa(i)}, now); err != nil {
			t.Fatalf("submission %d should pass, got %v", i, err)
		}
	}
	if err := s.CheckSubmission(Submission{IP: "1.2.3.4", Target: "post:z"}, now); err != ErrRateLimited {
		t.Fatalf("third per-IP submission should be rate limited, got %v", err)
	}

	// A different IP is unaffected by the first IP's exhaustion.
	if err := s.CheckSubmission(Submission{IP: "5.6.7.8", Target: "post:q"}, now); err != nil {
		t.Fatalf("different IP should pass, got %v", err)
	}
}

func TestWindowLimiter_Resets(t *testing.T) {
	l := newWindowLimiter(time.Minute)
	current := time.Unix(0, 0)
	l.now = func() time.Time { return current }

	if !l.allow("k", 1) {
		t.Fatal("first hit should pass")
	}
	if l.allow("k", 1) {
		t.Fatal("second hit in window should be blocked")
	}

	current = current.Add(61 * time.Second)
	if !l.allow("k", 1) {
		t.Fatal("hit after window reset should pass")
	}
}

func TestWindowLimiter_ZeroLimitUnlimited(t *testing.T) {
	l := newWindowLimiter(time.Minute)
	for i := 0; i < 1000; i++ {
		if !l.allow("k", 0) {
			t.Fatal("limit <= 0 should never block")
		}
	}
}
