package routes

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"google.golang.org/grpc/status"
	"net/http"
	"sync"
	"time"
)

var (
	candidatesCache = make(map[string]CandidateListLedger)
	mutex           sync.Mutex
)

type Candidate struct {
	Name       string `json:"name"`
	UserID     string `json:"userID"`
	ElectionID string `json:"electionID"`
}

type candidateState struct {
	ElectionID string `json:"election_id"`
}

type candidateElectionList struct {
	Name      string           `json:"name"`
	UserID    string           `json:"userID"`
	Elections []candidateState `json:"elections"`
}

type CandidateListLedger struct {
	Key    string `json:"Key"`
	Record struct {
		Elections []struct {
			ElectionID string `json:"electionID"`
			Votes      int    `json:"votes"`
		} `json:"elections"`
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"Record"`
}

func init() {
	// clear cache in interval
	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			mutex.Lock()
			candidatesCache = make(map[string]CandidateListLedger)
			mutex.Unlock()
		}
	}()

}

// @Summary Create Candidate
// @Description Create a new Candidate
// @Tags Candidate
// @Accept  json
// @Produce  json
// @Body  {object} name, userID, electionID, faculty, party, avatar
// @Success 200 {string} string "Candidate created"
// @Router /Candidate [post]
func createCandidate(contract *client.Contract, c *gin.Context) {

	// get Candidate studentName, userID and electionId from request body
	var candidate Candidate
	if err := c.ShouldBindJSON(&candidate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := contract.SubmitTransaction("createCandidate", candidate.Name, candidate.UserID, candidate.ElectionID)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")

	// retunr in json
	c.JSON(http.StatusCreated, gin.H{
		"message": "Candidate created. Txn committed successfully.",
		"status":  http.StatusCreated,
	})
}

// @Summary Get all Candidates
// @Description Get all candidates
// @Tags Candidate
// @Accept  json
// @Produce  json
// @Success 200 {string} string "Candidates fetched"
// @Router /Candidate [get]
func getAllCandidates(contract *client.Contract, c *gin.Context) {
	result, err := contract.EvaluateTransaction("queryByRange", "candidate.", "candidate.z")
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	fmt.Println(string(result))

	var response []CandidateListLedger
	err = json.Unmarshal(result, &response)
	if err != nil {
		c.JSON(http.StatusRequestTimeout, gin.H{"error": err.Error()})
		return
	}

	finalRes := make([]candidateElectionList, len(response))

	for i, c := range response {
		fmt.Println("tssss", c.Record)
		finalRes[i] = candidateElectionList{
			Name:   c.Record.Name,
			UserID: c.Key,
		}
		finalRes[i].Elections = make([]candidateState, len(c.Record.Elections))
		for j, e := range c.Record.Elections {
			finalRes[i].Elections[j] = candidateState{
				ElectionID: e.ElectionID,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Candidates fetched",
		"status":  http.StatusOK,
		"data":    finalRes,
	})
}

// @Summary Get Candidate
// @Description Get Candidate by electionID
// @Tags Candidate
// @Accept  json
// @Produce  json
// @Param electionID path string true "Election ID"
// @Success 200 {string} string "Candidates fetched"
// @Router /Candidate/{electionID} [get]
func getCandidatesByElectionId(contract *client.Contract, c *gin.Context) {
	electionID := c.Param("electionID")
	/*	if electionData, found := candidatesCache[electionID]; found {

		} else {

		}*/
	result, err := contract.EvaluateTransaction("getCandidatesById", electionID)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"detail": s.Details(), "message": s.Message()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	fmt.Printf("*** Transaction result: %s\n", string(result))

	var response []CandidateListLedger

	err = json.Unmarshal(result, &response)
	if err != nil {
		c.JSON(http.StatusRequestTimeout, gin.H{"error": err.Error()})
		panic(fmt.Errorf("failed to unmarshal JSON data: %w", err))
	}

	finalRes := make([]candidateElectionList, len(response))

	for i, c := range response {
		finalRes[i] = candidateElectionList{
			Name:   c.Record.Name,
			UserID: c.Key,
		}
		finalRes[i].Elections = make([]candidateState, len(c.Record.Elections))
		for j, e := range c.Record.Elections {
			finalRes[i].Elections[j] = candidateState{
				ElectionID: e.ElectionID,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Candidates fetched",
		"data":    finalRes,
		"status":  http.StatusOK,
	})
}
