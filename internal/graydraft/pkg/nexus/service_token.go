package nexus

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const agentServiceRole = "agent_service"

// ServiceTokenProvider mints HS256 agent_service JWTs using the shared Core secret.
type ServiceTokenProvider struct {
	Secret  string
	Subject string
	TTL     time.Duration
	Static  string

	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewServiceTokenProvider(secret, subject, static string, ttl time.Duration) *ServiceTokenProvider {
	if subject == "" {
		subject = "graydraft-bff"
	}
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &ServiceTokenProvider{Secret: secret, Subject: subject, TTL: ttl, Static: static}
}

// Bearer returns Authorization bearer material (may be empty when auth is disabled).
func (p *ServiceTokenProvider) Bearer() (string, error) {
	if p == nil {
		return "", nil
	}
	if p.Secret == "" {
		return p.Static, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	skew := p.TTL / 10
	if skew < 2*time.Minute {
		skew = 2 * time.Minute
	}
	if p.token != "" && now.Before(p.expires.Add(-skew)) {
		return p.token, nil
	}
	token, exp, err := issueAgentServiceToken(p.Secret, p.Subject, p.TTL)
	if err != nil {
		if p.Static != "" {
			return p.Static, nil
		}
		return "", err
	}
	p.token = token
	p.expires = exp
	return token, nil
}

func issueAgentServiceToken(secret, subject string, ttl time.Duration) (string, time.Time, error) {
	if secret == "" {
		return "", time.Time{}, fmt.Errorf("nexus: jwt secret required")
	}
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	exp := time.Now().Add(ttl)
	claims := jwt.MapClaims{
		"role": agentServiceRole,
		"sub":  subject,
		"exp":  exp.Unix(),
		"iat":  time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}
