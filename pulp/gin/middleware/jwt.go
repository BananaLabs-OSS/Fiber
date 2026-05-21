// Package middleware provides ready-made middleware for cells using
// the pulpgin router. It mirrors Potassium's auth middleware so
// existing handler code that reads "account_id" and "session_id" from
// the context works without changes.
//
//	r := pulpgin.New()
//	r.Use(middleware.JWTAuth(middleware.JWTConfig{Secret: []byte(secret)}))
//	r.GET("/me", handler)
package middleware

import (
	"fmt"
	"strings"

	pulpgin "github.com/BananaLabs-OSS/Fiber/pulp/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ErrorResponse is the standard error shape every BananaKit service
// returns. Matches Potassium's middleware.ErrorResponse byte-for-byte
// so handler code that constructs these literals compiles unchanged.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Claims matches the claims BananAuth issues across the ecosystem.
type Claims struct {
	jwt.RegisteredClaims
	AccountID string `json:"account_id"`
	SessionID string `json:"session_id"`
}

// JWTConfig configures the JWTAuth middleware.
type JWTConfig struct {
	Secret []byte
}

// JWTAuth returns pulpgin middleware that validates Bearer tokens and
// stashes the parsed account_id and session_id in the request context.
// Handlers read them via c.GetString("account_id") / .GetString("session_id").
func JWTAuth(cfg JWTConfig) pulpgin.HandlerFunc {
	return func(c *pulpgin.Context) {
		claims, err := extractAndValidate(c.GetHeader("Authorization"), cfg.Secret)
		if err != nil {
			c.AbortWithStatusJSON(401, pulpgin.H{"error": err.Error()})
			return
		}
		c.Set("account_id", claims.AccountID)
		c.Set("session_id", claims.SessionID)
		c.Next()
	}
}

// ServiceAuth returns middleware for service-to-service authentication
// via a shared X-Service-Token header. Used by internal endpoints the
// public should never hit directly.
func ServiceAuth(serviceSecret string) pulpgin.HandlerFunc {
	return func(c *pulpgin.Context) {
		token := c.GetHeader("X-Service-Token")
		if token == "" {
			c.AbortWithStatusJSON(401, pulpgin.H{"error": "missing service token"})
			return
		}
		if token != serviceSecret {
			c.AbortWithStatusJSON(401, pulpgin.H{"error": "invalid service token"})
			return
		}
		c.Next()
	}
}

// ParseToken validates a raw JWT and returns the decoded claims. Useful
// outside the HTTP chain (e.g. WebSocket upgrade) where JWTAuth doesn't
// apply.
func ParseToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func extractAndValidate(authHeader string, secret []byte) (*Claims, error) {
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, fmt.Errorf("invalid authorization format, expected: Bearer <token>")
	}
	return ParseToken(parts[1], secret)
}
