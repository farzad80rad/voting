// basic chain code
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	pb "github.com/hyperledger/fabric-protos-go/peer"
)

type VotingChaincode struct {
}

// init ledger with 4 voting cadidates
type candidate struct {
	Name      string         `json:"name"`
	ID        string         `json:"id"`
	Elections []electionInfo `json:"elections"`
}

type electionInfo struct {
	ElectionID string `json:"electionID"`
	Votes      int    `json:"votes"`
}

type voterV2 struct {
	ID              string            `json:"id"`
	ElectionHistory []ElectionHistory `json:"electionHistory"`
}

type ElectionHistory struct {
	ElectionID string `json:"electionID"`
	VotedTo    string `json:"votedTo"`
}

type election struct {
	ElectionID   string  `json:"electionID"`
	ElectionName string  `json:"electionName"`
	StartDate    string  `json:"startDate"`
	EndDate      string  `json:"endDate"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    *string `json:"updatedAt"`
}

func main() {
	err := shim.Start(new(VotingChaincode))
	if err != nil {
		fmt.Printf("Error starting Voting chaincode: %s", err)
	}
}

func (t *VotingChaincode) Init(_ shim.ChaincodeStubInterface) pb.Response {

	return shim.Success(nil)
}

// https://kctheservant.medium.com/chaincode-invoke-and-query-fabbe2757db0
// Invoke function
func (t *VotingChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()

	switch function {
	case "initLedger":
		return t.Init(stub)
	case "getFinalResult":
		return t.GetFinalResult(stub, args)
	case "vote":
		return t.voteV2(stub, args)
	case "createElection":
		return t.createElection(stub, args)
	case "createVoter":
		return t.createVoter(stub, args)
	case "createCandidate":
		return t.createCandidate(stub, args)
	case "getElectionById":
		return t.getElectionById(stub, args)
	case "getAllElections":
		return t.getAllElections(stub)
	case "updateElection":
		return t.updateElection(stub, args)
	case "getCandidatesById":
		return t.getCandidatesById(stub, args)
	case "getVoter":
		return t.getVoter(stub, args)
	case "queryByRange":
		return t.queryByRange(stub, args)
	default:
		fmt.Println("invoke did not find func: " + function) //error
		return shim.Error("Received unknown function invocation")
	}
}

// create voter function
func (t *VotingChaincode) createVoter(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}
	voterID := args[0]
	if !strings.HasPrefix(voterID, "voter.") {
		voterID = "voter." + args[0]
	}

	var newVoter = voterV2{ID: voterID, ElectionHistory: nil}

	// find voter in ledger
	dupeVoterAsBytes, err := stub.GetState(voterID)
	if err != nil {
		return shim.Error("Failed to get voter: " + voterID)
	}
	dupeVoter := voterV2{}
	// if voter exists, return error
	if dupeVoterAsBytes != nil {
		json.Unmarshal(dupeVoterAsBytes, &dupeVoter)
		if dupeVoter.ID == voterID {
			return shim.Error("Voter already exists")
		}
	}

	newVoterAsBytes, _ := json.Marshal(newVoter)
	err = stub.PutState(voterID, newVoterAsBytes)

	if err != nil {
		fmt.Println("Error creating voter")
		return shim.Error(err.Error())
	}
	fmt.Println("Voter created")
	return shim.Success(nil)

}

// get voter function
func (t *VotingChaincode) getVoter(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}
	voterID := args[0]
	if !strings.HasPrefix(voterID, "voter.") {
		voterID = "voter." + args[0]
	}
	// find voter in ledger
	dupeVoterAsBytes, err := stub.GetState(voterID)
	if err != nil {
		return shim.Error("Failed to get voter: " + voterID)
	}

	if dupeVoterAsBytes != nil {
		return shim.Success(dupeVoterAsBytes)
	}

	return shim.Error("not found")
}

// when vote is casted, the generated id is stored in the ledger
// if the generated id is found in the ledger, chek if election id exist (this means the voter has voted for this election)
// if generated id is not found in the ledger, create new voter and store in ledger
// if generated id is found in the ledger, but election id is not found, update the voter and store in ledger
// this means we need a new voter model, the current can only store one election id
// and its checked using hasVoted flag.
func (t *VotingChaincode) voteV2(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	VoterID := args[0]
	if !strings.HasPrefix(VoterID, "voter.") {
		VoterID = "voter." + args[0]
	}
	CandidateID := args[1]
	ElectionID := args[2]

	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}
	// find voter in ledger
	voterAsBytes, err := stub.GetState(VoterID)
	if err != nil {
		return shim.Error("Failed to get voter: " + VoterID)
	}

	if voterAsBytes == nil {
		fmt.Printf("voter not found")
		return shim.Error("voter not found")
	}

	voterInfo := voterV2{}
	err = json.Unmarshal(voterAsBytes, &voterInfo)
	if err != nil {
		fmt.Println("Failed to get voter: ", err)
		return shim.Error("Failed to unmarshal voter")
	}

	// if election id exist, return error
	for i := 0; i < len(voterInfo.ElectionHistory); i++ {
		if voterInfo.ElectionHistory[i].ElectionID == ElectionID && voterInfo.ElectionHistory[i].VotedTo != "" {
			fmt.Printf("Voter has already voted for this election")
			return shim.Error("Voter has already voted")
		}
	}

	// get election
	electionAsBytes, err := stub.GetState(ElectionID)
	election := election{}
	err = json.Unmarshal(electionAsBytes, &election)
	if err != nil {
		return shim.Error("Failed to get election: " + ElectionID)
	}

	// parse election end date to datetime
	electionEndDate, err := time.Parse(time.DateTime, strings.TrimSpace(election.EndDate))
	if err != nil {
		return shim.Error("Failed to parse election end date: " + election.EndDate)
	}
	// check if election has ended
	if time.Now().After(electionEndDate) {
		return shim.Error("Election has ended")
	}

	// update candidate votes
	candidateAsBytes, err := stub.GetState(CandidateID)
	if err != nil {
		return shim.Error("Failed to get candidate: " + CandidateID)
	}
	if candidateAsBytes == nil {
		return shim.Error("invalid candidate")
	}

	candidate := candidate{}
	err = json.Unmarshal(candidateAsBytes, &candidate)
	if err != nil {
		return shim.Error("Failed to get candidate: " + CandidateID)
	}
	contestedElection := candidate.Elections
	for i := 0; i < len(contestedElection); i++ {
		fmt.Println(contestedElection[i].ElectionID)
		if contestedElection[i].ElectionID == ElectionID {
			contestedElection[i].Votes++
		}
	}
	candidate.Elections = contestedElection
	candidateAsBytes, _ = json.Marshal(candidate)
	stub.PutState(CandidateID, candidateAsBytes)
	// candidate votes ledger updated when

	electionEligibility := ElectionHistory{ElectionID: ElectionID, VotedTo: CandidateID}
	// update voter ledger
	// if voter does not exist, create new voter

	// if voter exist, update voter
	voter := voterV2{}
	json.Unmarshal(voterAsBytes, &voter)
	voter.ElectionHistory = append(voter.ElectionHistory, electionEligibility)
	voterAsBytes, _ = json.Marshal(voter)
	err = stub.PutState(VoterID, voterAsBytes)
	if err != nil {
		fmt.Println("failed to put voter", err.Error())
		return shim.Error("failed to commit to network")
	}

	err = stub.PutState("record_"+ElectionID+"_"+VoterID, []byte(CandidateID))
	if err != nil {
		fmt.Println("failed to put history of election voting", err.Error())
		return shim.Error("failed to commit to network")
	}

	return shim.Success(nil)
}

// get election by id function
func (t *VotingChaincode) getElectionById(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}
	electionId := args[0]
	electionAsBytes, err := stub.GetState(electionId)
	if err != nil {
		return shim.Error("Failed to get election: " + electionId)
	}
	return shim.Success(electionAsBytes)
}

// get all created elections function
func (t *VotingChaincode) getAllElections(stub shim.ChaincodeStubInterface) pb.Response {
	resultsIterator, err := stub.GetStateByRange("elections", "elections.z")
	if err != nil {
		return shim.Error("Failed to get elections")
	}
	defer resultsIterator.Close()
	// buffer is a JSON array containing QueryResults
	elections := []election{}
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		var e election
		if err := json.Unmarshal(queryResponse.Value, &e); err != nil {
			return shim.Error("Failed to unmarshal the election")
		}
		elections = append(elections, e)
	}
	res, _ := json.Marshal(elections)
	return shim.Success(res)

}

// create election function
func (t *VotingChaincode) createElection(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 5 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}
	electionName := args[0]
	startDate := args[1]
	endDate := args[2]
	// electionID is pecified in the REST API server
	// hence all peers will have the same electionID

	electionID := args[3]
	if !strings.HasPrefix(electionID, "election.") {
		electionID = "election." + electionID
	}
	createdAt := args[4]

	// check if election name is provided

	// creating Id using current time broke the block
	// when smart contract is issued by REST API not all peers run the contract at the same time
	// this means peer01 will have a different electionID than peer02
	// hence endorsement will fail becase of key and value mismatch between peers
	// to circumvent this issue we need to specify the electionID in the REST API call
	// electionID := "election." + strconv.Itoa(time.Now().Nanosecond())
	// createdAt := time.Now().String()

	if startDate > endDate {
		return shim.Error("Invalid election dates")
	}

	// generate unique election id
	var election = &election{electionID, electionName, startDate, endDate, createdAt, nil}
	electionAsBytes, _ := json.Marshal(election)
	err := stub.PutState(electionID, electionAsBytes)
	if err != nil {
		fmt.Println("Error creating election")
		return shim.Error(err.Error())
	}

	fmt.Println("election creation successful %s", electionID)
	return shim.Success(nil)
}

// TODO: if cadidate exists, update candidate and append electionId to candidate.Elections
// else create candidate
// create candidate function
func (t *VotingChaincode) createCandidate(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}

	candidateName := args[0]
	// create a special ID for candidate by concatenating C_ and userID
	userID := "candidate." + args[1]
	electionId := args[2]

	// check if cadidate exist
	candidateAsBytes, err := stub.GetState(userID)
	if err != nil {
		return shim.Error("Failed to get candidate: " + userID)
	}

	electoinInfo, err := stub.GetState(electionId)
	if err != nil {
		return shim.Error("Failed to get election: " + electionId)
	}
	if electoinInfo == nil {
		return shim.Error("election not found ")
	}

	if candidateAsBytes != nil {
		// if candidate exists, update candidate and append electionId to candidate.Elections
		candidateInfo := candidate{}
		json.Unmarshal(candidateAsBytes, &candidateInfo)

		for _, e := range candidateInfo.Elections {
			// check if the election has been already included in candidate's elections
			if e.ElectionID == electionId {
				fmt.Println("already belongs to this election")
				return shim.Error("already belongs to this election")
			}
		}

		info := electionInfo{ElectionID: electionId, Votes: 0}
		candidateInfo.Elections = append(candidateInfo.Elections, info)
		candidateAsBytes, _ := json.Marshal(candidateInfo)
		err := stub.PutState(userID, candidateAsBytes)
		if err != nil {
			fmt.Println("Error updating candidate")
			return shim.Error(err.Error())
		}
		fmt.Println("candidate update successful %s", userID)
		return shim.Success(nil)
	} else {
		// else create candidate
		info := electionInfo{ElectionID: electionId, Votes: 0}
		candidateInfo := candidate{
			Name:      candidateName,
			Elections: []electionInfo{info}}
		candidateAsBytes, _ := json.Marshal(candidateInfo)
		err := stub.PutState(userID, candidateAsBytes)
		if err != nil {
			fmt.Println("Error creating candidate")
			return shim.Error(err.Error())
		}
		fmt.Println("candidate creation successful %s", userID)
		return shim.Success(nil)
	}

}

func (t *VotingChaincode) updateElection(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	electionId := args[0]
	target := args[1]
	value := args[2]

	electionAsBytes, err := stub.GetState(electionId)
	if err != nil {
		return shim.Error("Failed to get election: " + electionId)
	}

	election := election{}
	json.Unmarshal(electionAsBytes, &election)

	if target == "name" {
		election.ElectionName = value
	} else if target == "startDate" {
		election.StartDate = value
	} else if target == "endDate" {
		election.EndDate = value
	} else {
		return shim.Error("Invalid target")
	}

	electionAsBytes, _ = json.Marshal(election)
	err = stub.PutState(electionId, electionAsBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

// get candidates by id
func (t *VotingChaincode) getCandidatesById(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}
	electionId := args[0]

	// get candidates for election by id
	// electionid is stored in the candidate object
	// so you need to get all candidateId keys and get the candidate object
	// that match the electionId
	userIDsAsBytes, err := stub.GetStateByRange("candidate.", "candidate.z")
	if err != nil {
		return shim.Error("Failed to get candidate: " + electionId)
	}
	defer userIDsAsBytes.Close()
	// buffer is a JSON array containing QueryResults
	var buffer bytes.Buffer
	buffer.WriteString("[")
	bArrayMemberAlreadyWritten := false
	for userIDsAsBytes.HasNext() {
		queryResponse, err := userIDsAsBytes.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		candidateAsBytes, err := stub.GetState(queryResponse.Key)
		if err != nil {
			return shim.Error(err.Error())
		}
		candidate := candidate{}
		json.Unmarshal(candidateAsBytes, &candidate)
		for _, election := range candidate.Elections {
			if election.ElectionID == electionId {
				if bArrayMemberAlreadyWritten {
					buffer.WriteString(",")
				}
				buffer.WriteString("{\"Key\":")
				buffer.WriteString("\"")
				buffer.WriteString(queryResponse.Key)
				buffer.WriteString("\"")

				buffer.WriteString(", \"Record\":")
				// Record is a JSON object, so we write as-is
				buffer.WriteString(string(candidateAsBytes))
				buffer.WriteString("}")
				bArrayMemberAlreadyWritten = true
			}
		}
	}
	buffer.WriteString("]")
	return shim.Success(buffer.Bytes())
}

func (t *VotingChaincode) GetFinalResult(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}
	electionID := args[0]

	finalResult := make(map[string]int)

	startFrom := "record_" + electionID
	EndAt := "record_" + electionID + "_z"

iterateTillEnd:
	for {
		resultsIterator, err := stub.GetStateByRange(startFrom, EndAt)
		if err != nil {
			return shim.Error(err.Error())
		}
		defer resultsIterator.Close()

		if !resultsIterator.HasNext() {
			break
		}

		for resultsIterator.HasNext() {

			queryResponse, err := resultsIterator.Next()
			if err != nil {
				return shim.Error(err.Error())
			}
			fmt.Println("finalResualt query ", queryResponse.Key, string(queryResponse.Value))

			if queryResponse.Key == startFrom {
				if resultsIterator.HasNext() {
					continue
				} else {
					break iterateTillEnd
				}
			}
			startFrom = queryResponse.Key

			votedTo := string(queryResponse.Value)
			finalResult[votedTo]++
		}
	}
	response, err := json.Marshal(finalResult)
	if err != nil {
		fmt.Println("failed to marshal response", err)
		return shim.Error("failed to create response")
	}

	return shim.Success(response)
}

// query by range function
func (t *VotingChaincode) queryByRange(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 2 {
		return shim.Error("Incorrect number of arguments. Expecting 2")
	}
	startKey := args[0]
	endKey := args[1]
	resultsIterator, err := stub.GetStateByRange(startKey, endKey)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()
	// buffer is a JSON array containing QueryResults
	var buffer bytes.Buffer
	buffer.WriteString("[")
	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")
	fmt.Printf("- queryByRange queryResult:\n%s\n", buffer.String())
	return shim.Success(buffer.Bytes())
}
