package routes

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"google.golang.org/grpc/status"
	"net/http"
	"time"
)

type Election struct {
	ElectionID   string `json:"electionID"`
	ElectionName string `json:"electionName"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	UpdatedAt    string `json:"updatedAt"`
}

// update getFinalResult
// @Summary get final result of an Election
// @Description this API will iterate over the ledger to find the exact amount of votes given to each Candidate in an Election
// @Tags Election
// @Accept  json
// @Produce  json
// @Param electionID path string true "Election ID"
// @Success 200 {object} map "{'candidate1':10,'candidate2':1230}"
// @Router /Election/{electionID} [get]
func getFinalResult(contract *client.Contract, c *gin.Context) {
	electionID := c.Param("electionID")

	result, err := contract.SubmitTransaction("getFinalResult", electionID)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		if s, found := status.FromError(err); found {
			c.JSON(http.StatusBadRequest, gin.H{"error": s.Details()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to get transaction: %w", err))
	}

	var r map[string]int
	json.Unmarshal(result, &r)

	c.JSON(http.StatusOK, gin.H{
		"states": r,
	})
}

// @Summary Create Election
// @Description Create a new Election
// @Tags Election
// @Accept  json
// @Produce  json
// @Body  {object} Election
// @Success 200 {string} string "Election created"
// @Router /Election [post]
func createElection(contract *client.Contract, c *gin.Context) {

	var election Election
	if err := c.ShouldBindJSON(&election); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// generate electionID using timestamp
	// eg Election.1621234567
	// which translates to Election.<timestamp>
	currentTime := time.Now()
	electionID := fmt.Sprintf("election.%d", currentTime.Unix())
	// time in readable utc
	createdAt := currentTime.UTC().String()

	_, err := contract.SubmitTransaction("createElection", election.ElectionName, election.StartDate, election.EndDate, electionID, createdAt)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")

	c.JSON(http.StatusCreated, electionID)
}

// @Summary Get Election by id
// @Description Get Election by electionID
// @Tags Election
// @Accept  json
// @Produce  json
// @Param electionID path string true "Election ID"
// @Success 200 {string} string "Election created"
// @Router /Election/{electionID} [get]
func getElectionById(contract *client.Contract, c *gin.Context) {
	electionID := c.Param("electionID")
	result, err := contract.EvaluateTransaction("getElectionById", electionID)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	fmt.Printf("*** Transaction result: %s\n", string(result))

	var response interface{}
	err = json.Unmarshal(result, &response)
	if err != nil {
		c.JSON(http.StatusRequestTimeout, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to unmarshal JSON data: %w", err))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Election fetched",
		"data":    response,
		"status":  http.StatusOK,
	})
}

// @Summary Get All Elections
// @Description Get all elections
// @Tags Election
// @Accept  json
// @Produce  json
// @Success 200 {string} string "Elections fetched"
// @Router /Election [get]
func getAllElections(contract *client.Contract, c *gin.Context) {
	// get all elections using queryByRange function chaincode
	result, err := contract.EvaluateTransaction("queryByRange", "election.", "election.z")
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to query transaction: %w", err))
	}

	// Record represents a record that can either be an Election or a raw string for candidates
	type Record struct {
		Election  *Election `json:"Record,omitempty"`
		Candidate string    `json:"candidate,omitempty"`
	}

	// Data represents the structure of the entire JSON
	type Data struct {
		Key string `json:"Key"`
	}

	var data []Data
	err = json.Unmarshal(result, &data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to unmarshal JSON data: %w", err))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Elections fetched successfully.",
		"data":    data,
		"status":  http.StatusOK,
	})

}
