package routes

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

var jwtKey = []byte("my-very-secure-secret-key-1234567890")

type AuthCredentials struct {
	Users  map[string]string
	Admins map[string]string
}

// Claims struct to hold JWT payload
type Claims struct {
	UserID string `json:"userID"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Middleware for JWT validation
func JwtMiddleware(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		tokenStr = strings.TrimSpace(tokenStr)
		tokenStr = strings.Trim(tokenStr, "\"")
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, "Missing token")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, "Invalid token")
			return
		}

		// Check role authorization
		for _, role := range roles {
			if claims.Role == role {
				c.Set("userID", claims.UserID)
				c.Set("role", claims.Role)
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, "user does not have the access")
		return
	}

}

// Generate JWT Token
func GenerateToken(username, role string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: username,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func GetAuthHandler(users, admins map[string]string) func(c *gin.Context) {
	return func(c *gin.Context) {

		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}

		if err := c.ShouldBindJSON(&creds); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, "Invalid JSON")
			return
		}

		// Validate role
		if creds.Role != "admin" && creds.Role != "user" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, "Invalid role")
			return
		}

		if creds.Role == "admin" {
			// Validate user credentials
			if storedPassword, exists := admins[creds.Username]; !exists || storedPassword != creds.Password {
				c.AbortWithStatusJSON(http.StatusUnauthorized, "Invalid username or password")
				return
			}
		} else {
			// Validate user credentials
			if storedPassword, exists := users[creds.Username]; !exists || storedPassword != creds.Password {
				c.AbortWithStatusJSON(http.StatusUnauthorized, "Invalid username or password")
				return
			}
		}

		// Generate token
		token, err := GenerateToken(creds.Username, creds.Role)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, "Failed to generate token")
			return
		}

		c.JSON(http.StatusOK, token)
	}
}
