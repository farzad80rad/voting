package routes

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric-gateway/pkg/client"
)

func SetupRouter(contract *client.Contract, credentials AuthCredentials) *gin.Engine {
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
		v1.POST("/authenticate", GetAuthHandler(credentials.Users, credentials.Admins))
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
		v1.GET("/getFinalResult/:electionID", JwtMiddleware("admin"), func(context *gin.Context) {
			getFinalResult(contract, context)
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
