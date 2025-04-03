package routes

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric-gateway/pkg/client"
)

type election struct {
	ElectionID   string `json:"electionID"`
	ElectionName string `json:"electionName"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	UpdatedAt    string `json:"updatedAt"`
}

type voter struct {
	UserID string `json:"userID"`
}

type vote struct {
	CandidateID string `json:"candidateID"`
	ElectionID  string `json:"electionID"`
}

type Response struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

var jwtKey = []byte("my-very-secure-secret-key-1234567890")

// Simulated user database
var users = map[string]string{
	"9831025": "1234",
	"9831026": "1234",
	"9831027": "1234",
	"9831024": "1234",
}

var admins = map[string]string{
	"9831024": "1234",
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

				fmt.Println("has permision")
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

func Authenticate(c *gin.Context) {

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
		c.AbortWithStatusJSON(http.StatusUnauthorized, "Invalid username or password")
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

func SetupRouter(contract *client.Contract) *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	v1 := r.Group("/api/v1")
	{
		v1.POST("/authenticate", Authenticate)
		v1.GET("/ping", JwtMiddleware("user", "admin"), pong)
		v1.GET("/hello", JwtMiddleware("user", "admin"), helloWorld)
		v1.POST("/candidate", JwtMiddleware("admin"), func(c *gin.Context) {
			createCandidate(contract, c)
		})
		v1.GET("/candidate", JwtMiddleware("user", "admin"), func(c *gin.Context) {
			getAllCandidates(contract, c)
		})
		v1.GET("/candidate/:electionID", JwtMiddleware("user", "admin"), func(c *gin.Context) {
			getCandidatesByElectionId(contract, c)
		})
		v1.POST("/election", JwtMiddleware("admin"), func(c *gin.Context) {
			createElection(contract, c)
		})
		v1.GET("/election/:electionID", JwtMiddleware("user", "admin"), func(c *gin.Context) {
			getElectionById(contract, c)
		})
		v1.GET("/election", JwtMiddleware("user", "admin"), func(c *gin.Context) {
			getAllElections(contract, c)
		})
		v1.POST("/voter", JwtMiddleware("admin"), func(c *gin.Context) {
			createVoter(contract, c)
		})
		v1.GET("/voters", JwtMiddleware("user", "admin"), func(c *gin.Context) {
			getAllVoters(contract, c)
		})
		v1.GET("/voter/:voterID", JwtMiddleware("user", "admin"), func(context *gin.Context) {
			getVoter(contract, context)
		})
		v1.POST("/vote", JwtMiddleware("user", "admin"), func(c *gin.Context) {
			castVote(contract, c)
		})
	}
	return r
}

// write swagger endpoint for /ping
// @Summary Ping Pong
// @Description Returns pong
// @Tags Signal
// @Produce  text/plain
// @Success 200 {string} string "pong"
// @Router /ping [get]
func pong(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}

// @Summary Hello World
// @Description Returns hello world
// @Tags Signal
// @Produce  json
// @Success 200 {string} string "hello world"
// @Router /hello [get]
func helloWorld(c *gin.Context) {
	// retunr in json
	c.JSON(http.StatusOK, gin.H{
		"message": "hello world",
		"status":  http.StatusOK,
	})
}
