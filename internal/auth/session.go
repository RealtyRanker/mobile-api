package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SessionIssuer mints and verifies the app's own access tokens (short-lived
// HS256 JWTs carrying the user id) and opaque refresh tokens (long-lived
// random strings, stored hashed by the caller).
type SessionIssuer struct {
	secret    []byte
	accessTTL time.Duration
}

func NewSessionIssuer(secret string, accessTTL time.Duration) *SessionIssuer {
	return &SessionIssuer{secret: []byte(secret), accessTTL: accessTTL}
}

type accessClaims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

// NewAccessToken issues a signed access token for userID.
func (s *SessionIssuer) NewAccessToken(userID int) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTTL)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(accessClaims{Sub: strconv.Itoa(userID), Exp: expiresAt.Unix()})
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header + "." + payload
	sig := s.sign(signingInput)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), expiresAt, nil
}

// VerifyAccessToken checks the signature and expiry of an access token and
// returns the user id it was issued for.
func (s *SessionIssuer) VerifyAccessToken(token string) (int, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("malformed token")
	}
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return 0, fmt.Errorf("decoding signature: %w", err)
	}
	expected := s.sign(signingInput)
	if !hmac.Equal(sig, expected) {
		return 0, fmt.Errorf("invalid signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("decoding payload: %w", err)
	}
	var claims accessClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return 0, fmt.Errorf("parsing payload: %w", err)
	}
	if time.Now().Unix() >= claims.Exp {
		return 0, fmt.Errorf("token expired")
	}
	userID, err := strconv.Atoi(claims.Sub)
	if err != nil {
		return 0, fmt.Errorf("invalid subject: %w", err)
	}
	return userID, nil
}

func (s *SessionIssuer) sign(signingInput string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(signingInput))
	return mac.Sum(nil)
}

// NewRefreshToken generates a fresh opaque refresh token plus the hash that
// should be stored in the database (never the raw token itself).
func NewRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generating refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken derives the stored digest for a raw refresh token.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
