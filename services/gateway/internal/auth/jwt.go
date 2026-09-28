package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrMissingToken = errors.New("authorization header missing")
	ErrInvalidAuth  = errors.New("invalid authorization format")
)

const (
	defaultTokenExpiry = 24 * time.Hour
	issuer             = "prophet-gateway"
)

// Claims represents the JWT claims for a user session.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
	Role   string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// Config holds JWT configuration.
type Config struct {
	Secret     string
	Expiry     time.Duration
	Issuer     string
	Audience   []string
}

// LoadConfig loads JWT configuration from environment variables.
func LoadConfig() (*Config, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		// Generate a random secret for development only
		// In production, JWT_SECRET must be set
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate random secret: %w", err)
		}
		secret = hex.EncodeToString(b)
	}

	expiryStr := strings.TrimSpace(os.Getenv("JWT_EXPIRY"))
	expiry := defaultTokenExpiry
	if expiryStr != "" {
		parsed, err := time.ParseDuration(expiryStr)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_EXPIRY: %w", err)
		}
		expiry = parsed
	}

	audienceStr := strings.TrimSpace(os.Getenv("JWT_AUDIENCE"))
	audience := []string{"prophet-api"}
	if audienceStr != "" {
		audience = strings.Split(audienceStr, ",")
		for i, a := range audience {
			audience[i] = strings.TrimSpace(a)
		}
	}

	return &Config{
		Secret:   secret,
		Expiry:   expiry,
		Issuer:   issuer,
		Audience: audience,
	}, nil
}

// GenerateToken creates a new JWT for the given user.
func GenerateToken(cfg *Config, userID, email, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Audience:  jwt.ClaimStrings(cfg.Audience),
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.Expiry)),
			ID:        generateJTI(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ParseToken validates and parses a JWT token.
func ParseToken(cfg *Config, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	}, jwt.WithIssuer(cfg.Issuer), jwt.WithAudience(cfg.Audience...))

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ExtractTokenFromHeader extracts the bearer token from Authorization header.
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", ErrMissingToken
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidAuth
	}

	return strings.TrimSpace(parts[1]), nil
}

func generateJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405.000000")
	}
	return hex.EncodeToString(b)
}