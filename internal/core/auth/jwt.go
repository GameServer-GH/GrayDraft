package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	RoleAdmin = "admin"
	RoleAgent = "agent_service"
)

// Claims is the JWT payload for Console operators and agent service accounts.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Enabled reports whether JWT enforcement is active (secret configured).
func Enabled(secret string) bool {
	return strings.TrimSpace(secret) != ""
}

// Issue creates a bearer token for an admin subject (Console login).
func Issue(secret, subject string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	return IssueWithRole(secret, subject, RoleAdmin, ttl)
}

// IssueService creates a bearer token for an agent service account (BFF).
func IssueService(secret, subject string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	if strings.TrimSpace(subject) == "" {
		subject = "ghost-bff"
	}
	return IssueWithRole(secret, subject, RoleAgent, ttl)
}

// IssueWithRole creates a bearer token with an explicit role.
func IssueWithRole(secret, subject, role string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	if !Enabled(secret) {
		return "", time.Time{}, fmt.Errorf("jwt secret not configured")
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = RoleAdmin
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	expiresAt = time.Now().Add(ttl)
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// Validate parses and verifies a bearer token for admin Console access.
func Validate(secret, token string) (*Claims, error) {
	return ValidateRoles(secret, token, RoleAdmin)
}

// ValidateRoles parses a token and requires one of the allowed roles.
func ValidateRoles(secret, token string, roles ...string) (*Claims, error) {
	if !Enabled(secret) {
		return nil, fmt.Errorf("jwt secret not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if len(roles) == 0 {
		return claims, nil
	}
	for _, r := range roles {
		if claims.Role == r {
			return claims, nil
		}
	}
	return nil, fmt.Errorf("insufficient role")
}

// BearerToken extracts the token from an Authorization header.
func BearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
