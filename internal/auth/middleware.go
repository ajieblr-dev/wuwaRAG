package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ContextUserID   = "userID"
	ContextUsername = "username"
	ContextUserRole = "userRole"
	ContextClaims   = "claims"
)

// ExtractToken retrieves the bearer token from the Authorization header or cookie.
func ExtractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(authHeader)
	}

	if cookie, err := c.Cookie("ra_token"); err == nil && cookie != "" {
		return strings.TrimSpace(cookie)
	}

	if qToken := c.Query("token"); qToken != "" {
		return strings.TrimSpace(qToken)
	}

	return ""
}

// AuthRequired enforces that the request has a valid JWT bearer token.
func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ExtractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authentication required. Please sign in.",
			})
			return
		}

		claims, err := ValidateToken(jwtSecret, tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or expired session. Please sign in again.",
			})
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUsername, claims.Username)
		c.Set(ContextUserRole, claims.Role)
		c.Set(ContextClaims, claims)
		c.Next()
	}
}

// AdminRequired enforces that the request has a valid JWT and the role is 'admin'.
func AdminRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ExtractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authentication required. Please sign in as administrator.",
			})
			return
		}

		claims, err := ValidateToken(jwtSecret, tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or expired session. Please sign in again.",
			})
			return
		}

		if strings.ToLower(claims.Role) != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Admin privileges required to access this resource.",
			})
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUsername, claims.Username)
		c.Set(ContextUserRole, claims.Role)
		c.Set(ContextClaims, claims)
		c.Next()
	}
}

// OptionalAuth parses a JWT if present, but does not reject unauthenticated requests.
func OptionalAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ExtractToken(c)
		if tokenStr != "" {
			if claims, err := ValidateToken(jwtSecret, tokenStr); err == nil {
				c.Set(ContextUserID, claims.UserID)
				c.Set(ContextUsername, claims.Username)
				c.Set(ContextUserRole, claims.Role)
				c.Set(ContextClaims, claims)
			}
		}
		c.Next()
	}
}
