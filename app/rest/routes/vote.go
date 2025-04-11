package routes

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
)

type voter struct {
	UserID string `json:"userID"`
}

type Vote struct {
	CandidateID string `json:"candidateID"`
	ElectionID  string `json:"electionID"`
}

type Response struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type voterHistory struct {
	Id              string             `json:"id"`
	ElectionHistory []voterHistoryItem `json:"electionHistory"`
}

type voterHistoryItem struct {
	ElectionID string `json:"electionID"`
	VotedTo    string `json:"votedTo"`
}

type votersList struct {
	Key    string `json:"Key"`
	Record struct {
		ElectionHistory []struct {
			ElectionID string `json:"electionID"`
			VotedTo    string `json:"votedTo"`
		} `json:"electionHistory"`
		Id string `json:"id"`
	} `json:"Record"`
}

// @Summary Get All Voters
// @Description Get all voters
// @Tags Election
// @Accept  json
// @Produce  json
// @Success 200 {string} string "Elections fetched"
// @Router /voters [get]
func getAllVoters(contract *client.Contract, c *gin.Context) {
	// get all elections using queryByRange function chaincode
	result, err := contract.EvaluateTransaction("queryByRange", "voter.", "voter.z")
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to query transaction: %w", err))
	}

	fmt.Printf("*** Transaction result: %s\n", string(result))

	var response []votersList
	err = json.Unmarshal(result, &response)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to unmarshal JSON data: %w", err))
	}

	finalResp := make([]string, len(response))

	for i, list := range response {
		finalResp[i] = list.Key
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Elections fetched successfully.",
		"data":    finalResp,
		"status":  http.StatusOK,
	})

}

// @Summary Get single Voter
// @Description get full detail about a voter
// @Tags Election
// @Accept  json
// @Produce  json
// @Success 200 {string} string "Elections fetched"
// @Router /voters [get]
func getVoter(contract *client.Contract, c *gin.Context) {

	voterID := c.Param("voterID")
	if role, _ := c.Get("role"); role.(string) != "admin" {
		if userID, _ := c.Get("userID"); strings.TrimPrefix(userID.(string), "voter.") != strings.TrimPrefix(voterID, "voter.") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"err": "you cant see history of this voter"})
			return
		}
	}

	// get all elections using queryByRange function chaincode
	result, err := contract.EvaluateTransaction("getVoter", voterID)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to query transaction: %w", err))
	}

	fmt.Printf("*** Transaction result: %s\n", string(result))

	var response voterHistory
	err = json.Unmarshal(result, &response)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to unmarshal JSON data: %w", err))
	}

	type voterInfo struct {
		UserID  string             `json:"user_id"`
		History []voterHistoryItem `json:"history"`
	}

	fmt.Println(response)

	finalRes := voterInfo{
		UserID: response.Id,
	}

	finalRes.History = make([]voterHistoryItem, len(response.ElectionHistory))

	for i, s := range response.ElectionHistory {
		finalRes.History[i] = voterHistoryItem{
			ElectionID: s.ElectionID,
			VotedTo:    s.VotedTo,
		}
	}

	fmt.Println(finalRes)
	c.JSON(http.StatusOK, gin.H{
		"message": "Elections fetched successfully.",
		"data":    finalRes,
		"status":  http.StatusOK,
	})

}

// @Summary Create Voter
// @Description Create a new voter
// @Tags Voter
// @Accept  json
// @Produce  json
// @Body  {object} name, userID, electionID
// @Success 200 {string} string "Voter created"
// @Router /voter [post]
func createVoter(contract *client.Contract, c *gin.Context) {
	var voter voter
	if err := c.ShouldBindJSON(&voter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Println("this is voter", voter)

	_, err := contract.SubmitTransaction("createVoter", voter.UserID)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")

	c.JSON(http.StatusCreated, gin.H{
		"message": "Voter created. Txn committed successfully.",
		"status":  http.StatusCreated,
	})
}

// @Summary Vote
// @Description Vote for a Candidate
// @Tags Ballot
// @Accept  json
// @Produce  json
// @Body  {object} voterID, candidateID
// @Success 200 {string} string "Vote casted"
// @Router /ballot/Vote [post]
func castVote(contract *client.Contract, c *gin.Context) {

	var vote Vote
	if err := c.ShouldBindJSON(&vote); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, found := c.Get("userID")
	if !found {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "identity not found"})
		panic(fmt.Errorf("userID not found in contex"))
	}

	fmt.Println("vote Info", userID, vote)
	_, err := contract.SubmitTransaction("vote", userID.(string), vote.CandidateID, vote.ElectionID)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")

	c.JSON(http.StatusOK, gin.H{
		"message": "Vote casted. Txn committed successfully.",
		"status":  http.StatusOK,
	})
}
