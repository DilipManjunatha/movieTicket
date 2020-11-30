package main

// Assumption - Fixed showcode for all movie halls and theatres (ex: 9AM, 12PM, 3PM, 7PM)
// Assumption - Theatres will trigger reset(through  API) of available tickets and inventory for all shows every day before the first show
// Assumption - Movie names are in English and no Unicode characters
// Assumption - Max Sodas available per day is 200. Theatres have to reset the available count everyday
// Assumption - Theatre details will be added before adding screen-wise show details, selling tickets or exchanging soda

/* Sample Peer commands for various functions

peer chaincode invoke -n moviecc -c '{"args":["asd","{\"moviename\":\"B\", \"screen\":\"1\", \"thid\":\"pvrjp\", \"showcode\": [\"3\",\"2\"]}"]}' -C movieTheatre

peer chaincode invoke -n moviecc -c '{"args":["gss","{\"selector\": {\"thid\": \"pvrjp\"}}"]}' -C movieTheatre

peer chaincode invoke -n moviecc -c '{"args":["athd","{\"thid\":\"pvrjp\", \"sph\": {\"1\":\"100\",\"2\":\"100\",\"3\":\"100\",\"4\":\"100\",\"5\":\"100\"}}"]}' -C movieTheatre

peer chaincode invoke -n moviecc -c '{"args":["exs","{\"thid\":\"pvrjp\", \"inventoryid\": \"ES13\"]}"]}' -C myc

peer chaincode invoke -n mycc -c '{"args":["exs","{\"thid\": \"pvrjp\", \"movie\":\"Lucy\", \"screen\":\"1\", \"showtime\":\"2\", \"sold\":\"3\"}"]}' -C movieTheatre
*/

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

var _logger = shim.NewLogger("Shows-logger")

// ShowDetails maintains the show related details
type ShowDetails struct {
	ObjType   string   `json:"obj"`
	MovieName string   `json:"moviename"`
	Screen    string   `json:"screen"`
	TheatreID string   `json:"thid"`
	ShowCode  []string `json:"showcode"`
}

// TheatreDetails has movie hall-wise max capacity and inventory capacity details
type TheatreDetails struct {
	ObjType       string            `json:"obj"`
	TheatreID     string            `json:"thid"`
	SeatsPerHall  map[string]string `json:"sph"`
	MaxSodaPerDay string            `json:maxsoda`
}

// SodaInventory keeps track of day-wise soda sale.
type SodaInventory struct {
	ObjType     string `json:"obj"`
	TheatreID   string `json:"thid"`
	InventoryID string `json:"inventoryid"`
	SodaSold    string `json:"soda"`
}

// Tickets is the show-wise state data
type Tickets struct {
	ObjType     string `json:"obj"`
	TheatreID   string `json:"thid"`
	MovieName   string `json:"movie"`
	Screen      string `json:"screen"`
	ShowTime    string `json:"showcode"`
	TicketsSold string `json:"sold"`
	PopCornSold string `json:"pcsold"`
	WaterSold   string `json:"watersold"`
}

// ShowsManagement is the chaincode construct
type ShowsManagement struct {
}

var jsonResp string
var errorKey string
var errorData string

// Init Initialises the chaincode
func (s *ShowsManagement) Init(stub shim.ChaincodeStubInterface) pb.Response {
	_logger.Info("######### ShowsMangement is Initialized successfully #########")
	return shim.Success(nil)
}

// Invoke gets called when interacting with ledger
func (s *ShowsManagement) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	fn, args := stub.GetFunctionAndParameters()
	_logger.Info("ShowsMangement CC is invoked with function: ", string(fn))

	switch fn {
	case "asd":
		return s.addShowDetails(stub, args)
	case "exs":
		return s.exchangeSoda(stub, args)
	case "athd":
		return s.addTheatreDetails(stub, args)
	case "gss":
		return s.getShowDetails(stub, args)
	case "sell":
		return s.sellTicket(stub, args)
	default:
		_logger.Errorf("Unknown Function Invoked. Available Functions : asd,athd,gss,sell")
		jsonResp = "{\"Data\":" + fn + ",\"ErrorDetails\":\"Available Functions:asd,athd,gss,sell\"}"
		return shim.Error(jsonResp)
	}
}

// Add movie name and show timings for a particular screen in a theatre
func (s *ShowsManagement) addShowDetails(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		_logger.Info("addShowDetails: Incorrect number of arguments provided for the transaction.")
		jsonResp = "{\"Data\":" + strconv.Itoa(len(args)) + ",\"ErrorDetails\":\"Invalid Number of argumnets provided for transaction\"}"
		return shim.Error(jsonResp)
	}

	var sd ShowDetails
	err := json.Unmarshal([]byte(args[0]), &sd)
	if err != nil {
		errorKey = args[0]
		errorData = "Invalid json provided as input"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	sc := strings.ToLower(sd.Screen)
	thid := strings.ToLower(sd.TheatreID)

	// Check if the theatre details is added for the movie hall. If not present, do not process the request
	theatreDetails, err := stub.GetState(thid)

	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if theatreDetails == nil {
		errorData = "Theatre details does not exists for :" + string(thid)
		jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	compositeKey := thid + sc                       // Form composite key with TheatreID and ScreenCode
	movieExists, err := stub.GetState(compositeKey) // Check if the show details of a particular movie hall is added by a theatre

	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if movieExists != nil {
		errorData = "Show details already added by " + string(thid)
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	// Check if the movie-hall/screen details is present with the theatre
	td := TheatreDetails{}
	err = json.Unmarshal(theatreDetails, &td)
	if err != nil {
		errorData = "Existing theatre details Unmarshalling error"
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if td.SeatsPerHall[sc] == "" {
		errorData = "This screen details does not exists for the theatre"
		jsonResp = "{\"Data\":" + sc + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	sd.ObjType = "ShowDetails"
	sdjson, err := json.Marshal(sd)
	err = stub.PutState(compositeKey, sdjson)
	if err != nil {
		_logger.Errorf("addShowDetails:PutState is Failed :" + string(err.Error()))
		jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to add the show details\"}"
		return shim.Error(jsonResp)
	}
	_logger.Infof("addShowDetails:Show details added succesfully for theatre :" + string(thid))
	result := map[string]interface{}{
		"trxnid":  stub.GetTxID(),
		"message": "Add Show Detail Success",
	}
	respjson, _ := json.Marshal(result)
	return shim.Success(respjson)
}

func (s *ShowsManagement) getShowDetails(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting screen number to query")
	}

	var records []ShowDetails
	queryString := args[0]

	valAsbytes, err := stub.GetQueryResult(queryString)

	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + queryString + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"Record does not exist: " + queryString + "\"}"
		return shim.Error(jsonResp)
	}

	for valAsbytes.HasNext() {
		record := ShowDetails{}
		recordBytes, _ := valAsbytes.Next()
		if (string(recordBytes.Value)) == "" {
			continue
		}
		err = json.Unmarshal(recordBytes.Value, &record)
		if err != nil {
			replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
			errorData = "Unmarshalling Error :" + replaceErr
			jsonResp = "{\"Data\":" + string(recordBytes.Value) + ",\"ErrorDetails\":\"" + errorData + "\"}"
			return shim.Error(jsonResp)
		}
		records = append(records, record)
	}

	resultData := map[string]interface{}{
		"status":  "true",
		"records": records,
	}
	respjson, _ := json.Marshal(resultData)
	return shim.Success(respjson)
}

func (s *ShowsManagement) addTheatreDetails(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) != 1 {
		_logger.Info("addTheatreDetails: Incorrect number of arguments provided for the transaction.")
		jsonResp = "{\"Data\":" + strconv.Itoa(len(args)) + ",\"ErrorDetails\":\"Invalid Number of argumnets provided for transaction\"}"
		return shim.Error(jsonResp)
	}

	var td TheatreDetails
	err := json.Unmarshal([]byte(args[0]), &td)
	if err != nil {
		errorKey = args[0]
		errorData = "Invalid json provided as input"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addTheatreDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	thid := strings.ToLower(td.TheatreID)
	theatreExists, err := stub.GetState(thid) // Check if the theatre details is already added

	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addTheatreDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if theatreExists != nil {
		errorData = "Theatre details already added"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addTheatreDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	td.ObjType = "TheatreDetails"
	tdjson, err := json.Marshal(td)
	err = stub.PutState(thid, tdjson)

	if err != nil {
		_logger.Errorf("addTheatreDetails:PutState is Failed :" + string(err.Error()))
		jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to add theatre details\"}"
		return shim.Error(jsonResp)
	}
	_logger.Infof("addTheatreDetails:Theatre details added succesfully for :" + string(thid))
	result := map[string]interface{}{
		"trxnid":  stub.GetTxID(),
		"message": "Add Theatre Details Success",
	}
	respJSON, _ := json.Marshal(result)
	return shim.Success(respJSON)
}

func (s *ShowsManagement) sellTicket(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var compositeKey string

	if len(args) != 1 {
		_logger.Info("sellTicket: Incorrect number of arguments provided for the transaction.")
		jsonResp = "{\"Data\":" + strconv.Itoa(len(args)) + ",\"ErrorDetails\":\"Invalid Number of argumnets provided for transaction\"}"
		return shim.Error(jsonResp)
	}

	var tkt Tickets
	err := json.Unmarshal([]byte(args[0]), &tkt)
	if err != nil {
		errorKey = args[0]
		errorData = "Invalid json provided as input"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	thid := strings.ToLower(tkt.TheatreID)
	sc := strings.ToLower(tkt.Screen)
	st := tkt.ShowTime

	// Check if the show details are available
	compositeKey = thid + sc
	showDetails, err := stub.GetState(compositeKey)

	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if showDetails == nil {
		errorData = "Show details does not exists for the given details"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	_logger.Info("All Show's details on the current screen: " + string(showDetails))

	// Check if tickets sales already for any given showcode of particular movie-hall
	compositeKey = thid + sc + st
	tktIssueStarted, err := stub.GetState(compositeKey)
	if err != nil {
		errorData = "GetState is Failed"
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	// Get movie hall-wise maximux seat capacity
	th, err := stub.GetState(thid)
	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	td := TheatreDetails{}
	err = json.Unmarshal(th, &td)
	if err != nil {
		errorData = "Existing theatre details Unmarshalling error"
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	toSell, err := strconv.Atoi(tkt.TicketsSold)
	if err != nil {
		_logger.Errorf("sellTicket: String to integer converstion failed ")
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to sell the ticket\"}"
		return shim.Error(jsonResp)
	}
	maxtkt, err := strconv.Atoi(td.SeatsPerHall[sc])
	if err != nil {
		_logger.Errorf("sellTicket: String to integer converstion failed ")
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to sell the ticket\"}"
		return shim.Error(jsonResp)
	}

	if tktIssueStarted == nil {

		if toSell > maxtkt {
			_logger.Error("sellTicket: Enough tickets not available")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Enough tickets not available\"}"
			return shim.Error(jsonResp)
		}

		tkt.ObjType = "Tickets"
		tkt.WaterSold, tkt.PopCornSold = strconv.Itoa(toSell), strconv.Itoa(toSell)
		tktjson, err := json.Marshal(tkt)
		err = stub.PutState(compositeKey, tktjson)
		if err != nil {
			_logger.Errorf("sellTicket:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to sell the ticket\"}"
			return shim.Error(jsonResp)
		}
		_logger.Infof("sellTicket:Tickets sold successfully")
	} else {

		ticket := Tickets{}
		err := json.Unmarshal(tktIssueStarted, &ticket)
		if err != nil {
			errorData = "Existing ticket details Unmarshalling error"
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
			_logger.Error("sellTicket:" + string(jsonResp))
			return shim.Error(jsonResp)
		}

		tkt.ObjType = "Tickets"
		sold, err := strconv.Atoi(ticket.TicketsSold)
		if err != nil {
			_logger.Errorf("sellTicket: String to integer converstion failed ")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to sell the ticket\"}"
			return shim.Error(jsonResp)
		}

		if (sold + toSell) > maxtkt {
			_logger.Error("sellTicket: Enough tickets not available")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Enough tickets not available\"}"
			return shim.Error(jsonResp)
		}

		val := strconv.Itoa(sold + toSell)
		tkt.TicketsSold = val
		tkt.WaterSold, tkt.PopCornSold = val, val

		updatedTkt, err := json.Marshal(tkt)
		err = stub.PutState(compositeKey, updatedTkt)
		if err != nil {
			_logger.Errorf("sellTicket:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to sell the ticket\"}"
			return shim.Error(jsonResp)
		}

		_logger.Infof("sellTicket:Tickets sold successfully")
	}

	result := map[string]interface{}{
		"trxnid":     stub.GetTxID(),
		"ticketSold": toSell,
		"message":    "Sell ticket successfull",
	}
	respjson, _ := json.Marshal(result)
	return shim.Success(respjson)

}

func (s *ShowsManagement) exchangeSoda(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if !generateRandomNumber() {
		errorData = "Better luck next time. Cannot exchange soda"
		_logger.Infof("exchangeSoda:" + string(errorData))
		jsonResp = "{\"Data\":" + strconv.Itoa(len(args)) + ",\"ErrorDetails\":" + errorData + "}"
		return shim.Error(jsonResp)
	}

	if len(args) != 1 {
		_logger.Info("exchangeSoda: Incorrect number of arguments provided for the transaction.")
		jsonResp = "{\"Data\":" + strconv.Itoa(len(args)) + ",\"ErrorDetails\":\"Invalid Number of argumnets provided for transaction\"}"
		return shim.Error(jsonResp)
	}

	var soda SodaInventory
	err := json.Unmarshal([]byte(args[0]), &soda)
	if err != nil {
		errorKey = args[0]
		errorData = "Invalid json provided as input"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("exchangeSoda:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	// Check max soda count for the theatre per day
	thid := strings.ToLower(soda.TheatreID)
	invid := strings.ToLower(soda.InventoryID)

	theatreDetails, err := stub.GetState(thid)

	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("exchangeSoda:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if theatreDetails == nil {
		errorData = "Theatre details does not exists"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("exchangeSoda:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	td := TheatreDetails{}
	err = json.Unmarshal(theatreDetails, &td)
	if err != nil {
		errorData = "Existing theatre details Unmarshalling error"
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("exchangeSoda:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	maxSoda, err := strconv.Atoi(td.MaxSodaPerDay)
	if err != nil {
		_logger.Errorf("exchangeSoda: String to integer converstion failed ")
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to exchange soda\"}"
		return shim.Error(jsonResp)
	}

	compositeKey := thid + invid
	sodaDetails, err := stub.GetState(compositeKey)

	if err != nil {
		errorKey = compositeKey
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("exchangeSoda:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	if sodaDetails == nil {
		c, _ := strconv.Atoi(soda.SodaSold)
		c++
		soda.SodaSold = strconv.Itoa(c)
		sodajson, err := json.Marshal(soda)
		err = stub.PutState(compositeKey, sodajson)
		if err != nil {
			_logger.Errorf("exchangeSoda:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to exchange soda\"}"
			return shim.Error(jsonResp)
		}
		_logger.Infof("exchangeSoda:Soda exchange successfull")
	} else {

		sodainv := SodaInventory{}
		err := json.Unmarshal(sodaDetails, &sodainv)
		if err != nil {
			errorData = "Existing inventory details Unmarshalling error"
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
			_logger.Error("exchangeSoda:" + string(jsonResp))
			return shim.Error(jsonResp)
		}

		soda.ObjType = "SodaInventory"
		sold, err := strconv.Atoi(sodainv.SodaSold)
		if err != nil {
			_logger.Errorf("exchangeSoda: String to integer converstion failed ")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to exchange soda\"}"
			return shim.Error(jsonResp)
		}

		if (sold + 1) > maxSoda {
			_logger.Error("exchangeSoda: Enough soda not available")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\" Enough soda not available\"}"
			return shim.Error(jsonResp)
		}

		c, _ := strconv.Atoi(soda.SodaSold)
		c++
		soda.SodaSold = strconv.Itoa(c)
		updatedinv, err := json.Marshal(soda)
		err = stub.PutState(compositeKey, updatedinv)
		if err != nil {
			_logger.Errorf("exchangeSoda:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to exchange the soda\"}"
			return shim.Error(jsonResp)
		}

		_logger.Infof("exchangeSoda:Soda exchange successfull")
	}

	result := map[string]interface{}{
		"trxnid":        stub.GetTxID(),
		"sodaExchanged": 1,
		"message":       "Soda exchange successfull",
	}
	respjson, _ := json.Marshal(result)
	return shim.Success(respjson)
}

func generateRandomNumber() bool {

	// Generate Random number
	min := 10
	max := 100
	rand.Seed(time.Now().UnixNano())
	randomNum := rand.Intn(max-min) + min
	if randomNum%2 == 0 {
		fmt.Printf("Random Num: %d\n", randomNum)
		return true
	}
	fmt.Printf("Random Num: %d\n", randomNum)
	return false

}

func main() {
	err := shim.Start(new(ShowsManagement))
	_logger.SetLevel(shim.LogDebug)
	if err != nil {
		_logger.Error("Error occured while starting the Shows chaincode")
	} else {
		_logger.Info("Starting the Shows chaincode")
	}
}
