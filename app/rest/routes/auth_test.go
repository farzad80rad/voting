package routes

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJwtMiddleware(t *testing.T) {
	// Create a test router
	r := gin.New()
	r.GET("/", JwtMiddleware("testRole"), func(c *gin.Context) {
		fmt.Println("tssss")
		fmt.Println(c.Get("role"))
		c.JSON(http.StatusOK, gin.H{"message": "Hello World"})
	})

	// Create a test token
	tokenStr, err := GenerateToken("testUser", "testRole")
	if err != nil {
		t.Fatal(err)
	}
	// Test with valid token
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Test with invalid token
	req, err = http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "invalidToken")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}

	// Test with missing token
	req, err = http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestGenerateToken(t *testing.T) {
	// Test with valid input
	tokenStr, err := GenerateToken("testUser", "testRole")
	if err != nil {
		t.Fatal(err)
	}
	if tokenStr == "" {
		t.Errorf("expected non-empty token string, got empty string")
	}

	// Test with invalid input
	_, err = GenerateToken("", "")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestAuthenticate(t *testing.T) {
	// Create a test router
	r := gin.New()

	// Test with valid input
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer([]byte(`{"username":"testUser","password":"testPassword","role":"user"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.POST("/", GetAuthHandler(map[string]string{"testUser": "testPassword"}, nil))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Test with invalid input
	req, err = http.NewRequest("POST", "/", bytes.NewBuffer([]byte(`{"username":"","password":"","role":""}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestJwtMiddlewareIntegration(t *testing.T) {
	// Create a test router
	r := gin.New()

	// Create a test token
	tokenStr, err := GenerateToken("testUser", "testRole")
	if err != nil {
		t.Fatal(err)
	}
	anotherRole, err := GenerateToken("testUser", "anotherRole")
	if err != nil {
		t.Fatal(err)
	}

	// Test with valid token
	req, err := http.NewRequest("GET", "/protected", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", tokenStr)
	w := httptest.NewRecorder()
	r.Use(JwtMiddleware("testRole"))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello World"})
	})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Test with invalid token
	req, err = http.NewRequest("GET", "/protected", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", anotherRole)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}

	// Test with missing token
	req, err = http.NewRequest("GET", "/protected", nil)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
