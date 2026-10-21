package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidToken indicates the JWT signature or claims are invalid.
	ErrInvalidToken = errors.New("invalid or expired authentication token")
	// ErrEmptyPassword indicates the provided password string is empty.
	ErrEmptyPassword = errors.New("password cannot be empty")
)

// Claims represents the JWT payload claims for an authenticated user.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // "user" or "admin"
	jwt.RegisteredClaims
}

// HashPassword creates a bcrypt hash of the given plain password.
func HashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrEmptyPassword
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword compares a bcrypt hashed password with its possible plaintext equivalent.
func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// GenerateToken issues a signed JWT token containing user identity and role.
func GenerateToken(jwtSecret, userID, username, role string, ttl time.Duration) (string, error) {
	if jwtSecret == "" {
		return "", errors.New("jwt secret cannot be empty")
	}
	if ttl == 0 {
		ttl = 7 * 24 * time.Hour // Default 7 days
	}

	now := time.Now().UTC()
	issuedAt := now
	notBefore := now
	if ttl < 0 {
		issuedAt = now.Add(ttl * 2)
		notBefore = issuedAt
	}
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(notBefore),
			Subject:   userID,
			Issuer:    "wuwaRAG",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken parses and validates a signed JWT token string.
func ValidateToken(jwtSecret, tokenStr string) (*Claims, error) {
	if jwtSecret == "" {
		return nil, errors.New("jwt secret cannot be empty")
	}

	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
