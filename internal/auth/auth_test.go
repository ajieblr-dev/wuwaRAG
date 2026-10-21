package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPasswordHashing(t *testing.T) {
	password := "my-secret-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == password {
		t.Fatal("Hash must not equal plaintext password")
	}

	if !CheckPassword(hash, password) {
		t.Fatal("CheckPassword should return true for valid password")
	}

	if CheckPassword(hash, "wrong-password") {
		t.Fatal("CheckPassword should return false for incorrect password")
	}

	_, err = HashPassword("")
	if err != ErrEmptyPassword {
		t.Errorf("Expected ErrEmptyPassword, got %v", err)
	}
}

func TestTokenGenerationAndValidation(t *testing.T) {
	secret := "test-secret-key-1234567890"
	userID := "usr-123"
	username := "solaris_admin"
	role := "admin"

	token, err := GenerateToken(secret, userID, username, role, time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != userID || claims.Username != username || claims.Role != role {
		t.Fatalf("Claims mismatch: %+v", claims)
	}

	// Invalid secret
	_, err = ValidateToken("wrong-secret", token)
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken for wrong secret, got %v", err)
	}

	// Expired token
	expiredToken, err := GenerateToken(secret, userID, username, role, -time.Minute)
	if err != nil {
		t.Fatalf("GenerateToken for expired token failed: %v", err)
	}
	_, err = ValidateToken(secret, expiredToken)
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret-jwt"

	userToken, _ := GenerateToken(secret, "user-1", "normal_user", "user", time.Hour)
	adminToken, _ := GenerateToken(secret, "admin-1", "admin_user", "admin", time.Hour)

	r := gin.New()
	r.GET("/protected", AuthRequired(secret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user": c.GetString(ContextUsername)})
	})
	r.GET("/admin-only", AdminRequired(secret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"admin": c.GetString(ContextUsername)})
	})

	// 1. Unauthenticated request to /protected
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for no token, got %d", w.Code)
	}

	// 2. User token to /protected -> 200 OK
	req, _ = http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for valid user token, got %d", w.Code)
	}

	// 3. User token to /admin-only -> 403 Forbidden
	req, _ = http.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for user token on admin route, got %d", w.Code)
	}

	// 4. Admin token to /admin-only -> 200 OK
	req, _ = http.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for admin token on admin route, got %d", w.Code)
	}
}
