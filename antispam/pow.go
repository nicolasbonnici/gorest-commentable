package antispam

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math/bits"
	"strconv"
	"strings"
	"time"
)

// Challenge token wire format: nonce.ts.bits.sig
//   nonce - random hex (client-opaque)
//   ts    - unix seconds the challenge was issued
//   bits  - required leading zero bits
//   sig   - hex HMAC-SHA256(secret, "nonce.ts.bits")
//
// The token is self-contained and signed, so no server-side storage is needed:
// validity, age and difficulty are all recovered and verified from the token.

func challengePayload(nonce string, ts int64, powBits int) string {
	return nonce + "." + strconv.FormatInt(ts, 10) + "." + strconv.Itoa(powBits)
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func issueChallenge(secret []byte, powBits int, now time.Time) string {
	nonce := randomHex(16)
	ts := now.Unix()
	payload := challengePayload(nonce, ts, powBits)
	return payload + "." + sign(secret, payload)
}

// verifyChallenge validates the token signature, age and embedded proof of work.
func verifyChallenge(secret []byte, token, solution string, now time.Time, ttl, minFill time.Duration) error {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return ErrChallengeFormat
	}
	nonce, tsStr, bitsStr, sig := parts[0], parts[1], parts[2], parts[3]
	if nonce == "" {
		return ErrChallengeFormat
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return ErrChallengeFormat
	}
	powBits, err := strconv.Atoi(bitsStr)
	if err != nil || powBits < 0 {
		return ErrChallengeFormat
	}

	payload := challengePayload(nonce, ts, powBits)
	expected := sign(secret, payload)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) != 1 {
		return ErrBadSignature
	}

	age := now.Sub(time.Unix(ts, 0))
	if ttl > 0 && age > ttl {
		return ErrExpired
	}
	if age < 0 {
		// Clock skew: a challenge from the future is not yet valid.
		return ErrExpired
	}
	if minFill > 0 && age < minFill {
		return ErrTooFast
	}

	if powBits > 0 && !meetsDifficulty(payload, solution, powBits) {
		return ErrInsufficientWork
	}

	return nil
}

// meetsDifficulty reports whether sha256(payload + "." + solution) begins with at
// least powBits zero bits.
func meetsDifficulty(payload, solution string, powBits int) bool {
	sum := sha256.Sum256([]byte(payload + "." + solution))
	return leadingZeroBits(sum[:]) >= powBits
}

func leadingZeroBits(b []byte) int {
	count := 0
	for _, c := range b {
		if c == 0 {
			count += 8
			continue
		}
		count += bits.LeadingZeros8(c)
		break
	}
	return count
}
