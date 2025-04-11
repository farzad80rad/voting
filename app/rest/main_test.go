package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	routers "github.com/izqalan/fabric-voting/app/routes"
	"github.com/spf13/cast"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	usersToken   = make([]string, 5000)
	adminToken   string
	r            *gin.Engine
	invalidToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySUQiOiJhZG1pbiIsInJvbGUiOiJhZG1pbiIsImV4cCI6MTc0Mzc3MDgxNX0.7Pws3oI3mr-uZzvgJETQlMYfm73TDvAYV5XDdpc3fEI" // this is a valid jwt token, but not signed with this service secret key.
)

func TestMain(t *testing.M) {
	// The gRPC client connection should be shared by all Gateway connections to this endpoint
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	id := newIdentity()
	sign := newSign()

	// Create a Gateway connection for a specific client identity
	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(clientConnection),
		// Default timeouts for different gRPC calls
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		panic(err)
	}
	defer gw.Close()

	// Override default values for chaincode and channel name as they may differ in testing contexts.
	chaincodeName := "mychaincode"
	if ccname := os.Getenv("CHAINCODE_NAME"); ccname != "" {
		chaincodeName = ccname
	}

	channelName := "mychannel"
	if cname := os.Getenv("CHANNEL_NAME"); cname != "" {
		channelName = cname
	}

	network := gw.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)

	usersName := make([]string, len(usersToken))
	mapUsersName := make(map[string]string)
	for i, _ := range usersName {
		usersName[i] = "user" + cast.ToString(i)
		mapUsersName[usersName[i]] = usersName[i]
	}

	// Rest Endpoints
	r = routers.SetupRouter(contract, routers.AuthCredentials{mapUsersName, map[string]string{"admin": "admin"}})

	// Swagger Endpoints
	r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	go r.Run(":80")

	for i, name := range usersName {
		req, err := http.NewRequest("POST", "/api/v1/authenticate", bytes.NewBuffer([]byte(`{"username":"`+name+`","password":"`+name+`","role":"user"}`)))
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var token string
		json.Unmarshal(w.Body.Bytes(), &token)

		usersToken[i] = token
	}
	//get user token

	//get admin token
	req, err := http.NewRequest("POST", "/api/v1/authenticate", bytes.NewBuffer([]byte(`{"username":"admin","password":"admin","role":"admin"}`)))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body, err := io.ReadAll(w.Body)
	if err != nil {
		panic(err)
	}
	adminToken = string(body)

	var wg sync.WaitGroup
	wg.Add(len(usersName))
	// register voters
	for _, name := range usersName {
		go func(name string) {
			defer wg.Done()
			req, err := http.NewRequest("POST", "/api/v1/voter", bytes.NewBuffer([]byte(`{"userID":"`+name+`"}`)))
			if err != nil {
				fmt.Println("failed to submit user", err.Error())
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", adminToken)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				fmt.Println("failed to submit user", w.Code, w.Body.String())
			}
		}(name)
	}
	wg.Wait()

	code := t.Run()
	os.Exit(code)
}

func createElection() string {

	var electionID string
	// register a new election
	{

		requestBody := routers.Election{
			ElectionName: "test",
			StartDate:    time.Now().Add(-24 * time.Hour).Format(time.DateTime),
			EndDate:      time.Now().Add(24 * time.Hour).Format(time.DateTime),
		}

		b, _ := json.Marshal(requestBody)
		req, err := http.NewRequest("POST", "/api/v1/election", bytes.NewBuffer(b))
		if err != nil {
			fmt.Println("failed to submit election", err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", adminToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			fmt.Println("failed to submit election", w.Code)
		}
		electionID = strings.Trim(w.Body.String(), "\"")
	}

	// register 2 new candidate for election
	for i := 1; i <= 2; i++ {
		requestBody := routers.Candidate{
			Name:       "candidate" + cast.ToString(i),
			UserID:     "candidate" + cast.ToString(i),
			ElectionID: electionID,
		}

		b, _ := json.Marshal(requestBody)
		req, err := http.NewRequest("POST", "/api/v1/candidate", bytes.NewBuffer(b))
		if err != nil {
			fmt.Println("failed to submit election", err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", adminToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			fmt.Println("failed to submit election", w.Code)
		}
	}

	return electionID
}

func TestInjectionCodeCreateElection(t *testing.T) {

	tests := []routers.Election{
		{
			ElectionName: "election.'); DROP TABLE elections;--",
			EndDate:      "<script>alert(1)</script>",
			StartDate:    "2020-01-01",
			UpdatedAt:    "2030-01-01",
		},
		{
			ElectionName: "election.🔥🔥🔥",
			StartDate:    "🔥🔥🔥",
			EndDate:      "🔥🔥",
			UpdatedAt:    "🔥🔥🔥🔥🔥🔥",
		},
		{
			ElectionName: "",
			StartDate:    "",
			EndDate:      "",
			UpdatedAt:    "",
		},
		{
			ElectionName: strings.Repeat("verylongstring", 1000),
			StartDate:    "invalid-date",
			EndDate:      "also-invalid",
		},
	}
	// register a new election
	for _, test := range tests {
		b, _ := json.Marshal(test)
		req, err := http.NewRequest("POST", "/api/v1/election", bytes.NewBuffer(b))
		if err != nil {
			fmt.Println("failed to submit election", err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", adminToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			fmt.Println("failed to submit election", w.Code)
		}
	}
}

func TestInjectionVote(t *testing.T) {

	electionID := createElection()

	tests := []routers.Vote{
		{
			CandidateID: "election.'); DROP;",
			ElectionID:  "candidate.1",
		},
		{
			CandidateID: "",
			ElectionID:  "",
		},
		{
			CandidateID: "candidate.🔥",
			ElectionID:  "election.🔥",
		},
		{
			CandidateID: "candidate.🔥",
			ElectionID:  electionID,
		},
	}
	// register a new election
	for i, test := range tests {
		jsonVote, err := json.Marshal(test)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", "/api/v1/vote", bytes.NewBuffer(jsonVote))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", usersToken[i])

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		fmt.Println(w.Body.String())
	}
}

func TestVote(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(len(usersToken))
	votesToFirst := 3
	electionID := createElection()
	for i, voterToken := range usersToken {
		go func(i int, voterToken string) {
			defer wg.Done()
			candid := "candidate1"
			if i >= votesToFirst {
				candid = "candidate2"
			}
			vote := routers.Vote{
				CandidateID: candid,
				ElectionID:  electionID,
			}
			jsonVote, err := json.Marshal(vote)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/api/v1/vote", bytes.NewBuffer(jsonVote))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", voterToken)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				fmt.Println(w.Body.String())
				t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
			}
		}(i, voterToken)
	}
	wg.Wait()

	//read result
	req, err := http.NewRequest("GET", "/api/v1/getFinalResult/"+electionID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", adminToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
		return
	}

	var resTemp struct {
		States map[string]int `json:"states"`
	}
	json.Unmarshal(w.Body.Bytes(), &resTemp)
	if votesToFirst != resTemp.States["candidate.candidate1"] {
		t.Error("not equal voting1")
		return
	}

	if len(usersToken)-votesToFirst != resTemp.States["candidate.candidate2"] {
		t.Error("not equal voting2")
		return
	}
}

func TestDuplicateRequest(t *testing.T) {
	electionID := createElection()

	for i := 0; i < 2; i++ {
		candid := "candidate1"
		vote := routers.Vote{
			CandidateID: candid,
			ElectionID:  electionID,
		}
		jsonVote, err := json.Marshal(vote)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest("POST", "/api/v1/vote", bytes.NewBuffer(jsonVote))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", usersToken[0])

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if i == 0 {
			if w.Code != http.StatusOK {
				fmt.Println(w.Body.String())
				t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
			}
		} else {
			if w.Code != http.StatusBadRequest {
				fmt.Println(w.Body.String())
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, w.Code)
			}
		}
	}
}

func TestVoteVisibility(t *testing.T) {
	wg := sync.WaitGroup{}
	electionID := createElection()
	for i, voterToken := range usersToken[:2] {
		wg.Add(1)
		go func(i int, voterToken string) {
			defer wg.Done()
			candid := "candidate1"
			vote := routers.Vote{
				CandidateID: candid,
				ElectionID:  electionID,
			}
			jsonVote, err := json.Marshal(vote)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/api/v1/vote", bytes.NewBuffer(jsonVote))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", voterToken)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				fmt.Println(w.Body.String())
				t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
			}

			//get self result
			req, err = http.NewRequest("GET", "/api/v1/voter/user"+cast.ToString(i), nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", voterToken)

			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
				return
			}
		}(i, voterToken)
	}
	wg.Wait()

	//get another user result
	req, err := http.NewRequest("GET", "/api/v1/voter/user1", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", usersToken[0]) // token is for user0 but request is for user1

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status code %d, got %d", http.StatusForbidden, w.Code)
		return
	}
}

func TestConcurrentRequestForSameCandid(t *testing.T) {
	electionID := createElection()

	candid := "candidate1"

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vote := routers.Vote{
				CandidateID: candid,
				ElectionID:  electionID,
			}
			jsonVote, err := json.Marshal(vote)
			if err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequest("POST", "/api/v1/vote", bytes.NewBuffer(jsonVote))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", usersToken[0])

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}()
	}

	wg.Wait()
	//read result
	req, err := http.NewRequest("GET", "/api/v1/getFinalResult/"+electionID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", adminToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
		return
	}

	var resTemp struct {
		States map[string]int `json:"states"`
	}
	json.Unmarshal(w.Body.Bytes(), &resTemp)
	if 1 != resTemp.States["candidate.candidate1"] {
		t.Error("only one voting should be accepted", resTemp.States["candidate.candidate1"])
		return
	}

}

func TestConcurrentRequestForMultiCandidates(t *testing.T) {
	electionID := createElection()

	var wg sync.WaitGroup
	for i := 0; i < 7; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			vote := routers.Vote{
				CandidateID: "candidate" + cast.ToString((i%2)+1),
				ElectionID:  electionID,
			}
			jsonVote, err := json.Marshal(vote)
			if err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequest("POST", "/api/v1/vote", bytes.NewBuffer(jsonVote))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", usersToken[0])

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}(i)
	}

	wg.Wait()
	//read result
	req, err := http.NewRequest("GET", "/api/v1/getFinalResult/"+electionID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", adminToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
		return
	}

	var resTemp struct {
		States map[string]int `json:"states"`
	}
	json.Unmarshal(w.Body.Bytes(), &resTemp)
	totalVotes := 0
	for _, i := range resTemp.States {
		totalVotes += i
	}
	if 1 != totalVotes {
		t.Error("only one voting should be accepted", totalVotes)
		return
	}
}

func TestInvalidUserVoting(t *testing.T) {
	electionID := createElection()

	candid := "candidate1"
	vote := routers.Vote{
		CandidateID: candid,
		ElectionID:  electionID,
	}
	jsonVote, err := json.Marshal(vote)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/api/v1/vote", bytes.NewBuffer(jsonVote))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", invalidToken)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		fmt.Println(w.Body.String())
		t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}

}

func TestAdminAccess(t *testing.T) {
	electionID := createElection()

	type inputTemplate struct {
		token string
	}
	type outputTemplate struct {
		statusCode int
	}
	type testInfo struct {
		name           string
		input          []inputTemplate
		operate        func(i inputTemplate) outputTemplate
		expectedOutput []outputTemplate
	}

	tests := []testInfo{
		{
			name: "check create voters",
			input: []inputTemplate{
				{token: usersToken[0]},
				{token: adminToken},
			},
			expectedOutput: []outputTemplate{
				{statusCode: http.StatusUnauthorized},
				{statusCode: http.StatusCreated},
			},
			operate: func(i inputTemplate) outputTemplate {
				name := "testingUser" + cast.ToString(time.Now().Unix())
				req, err := http.NewRequest("POST", "/api/v1/voter", bytes.NewBuffer([]byte(`{"userID":"`+name+`"}`)))
				if err != nil {
					fmt.Println("failed to submit user", err.Error())
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", i.token)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				return outputTemplate{statusCode: w.Code}

			},
		},
		{
			name: "check create candidate",
			input: []inputTemplate{
				{token: usersToken[0]},
				{token: adminToken},
			},
			expectedOutput: []outputTemplate{
				{statusCode: http.StatusUnauthorized},
				{statusCode: http.StatusCreated},
			},
			operate: func(i inputTemplate) outputTemplate {
				requestBody := routers.Candidate{
					Name:       "testingCandidate",
					UserID:     "testingCandidate",
					ElectionID: electionID,
				}
				b, _ := json.Marshal(requestBody)
				req, err := http.NewRequest("POST", "/api/v1/candidate", bytes.NewBuffer(b))
				if err != nil {
					fmt.Println("failed to submit election", err.Error())
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", i.token)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				return outputTemplate{statusCode: w.Code}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for i, input := range test.input {
				output := test.operate(input)
				if test.expectedOutput[i].statusCode != output.statusCode {
					t.Errorf("expected %d got %d", test.expectedOutput[i].statusCode, output.statusCode)
				}
			}
		})
	}
}
