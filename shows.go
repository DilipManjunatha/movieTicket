package main

// Assumption - Fixed showcode for all movie halls and theatres (ex: "1", "2", "3", "4")
// Assumption - Theatres will trigger reset(through  API) of available tickets and inventory for all shows every day before the first show
// Assumption - Movie names are in English and no Unicode characters
// Assumption - Max Sodas available per day is 200. Theatres have to reset the available count everyday
// Assumption - Theatre details will be added before adding screen-wise show details, selling tickets or exchanging soda
// Assumtion - Only 1 cafeteria inventory per theatre
// Assumption - All the mandatory parameters will be provided as per the function requirement i,e non-null values.

// All inputs are case sensitive
// More than one theatre can add the data on to Blockchain

/********************* Sample Peer commands for various functions ************************************

peer chaincode invoke -n moviecc -c '{"args":["asd","{\"moviename\":\"Lucy\", \"screen\":\"1\", \"thid\":\"Theatre1\", \"showcode\": [\"1\",\"2\",\"3\",\"4\"]}"]}' -C movieTheatre

peer chaincode invoke -n moviecc -c '{"args":["gss","{\"selector\": {\"thid\": \"Theatre1\"}}"]}' -C movieTheatre

peer chaincode invoke -n moviecc -c '{"args":["athd","{\"thid\":\"Theatre1\", \"maxsoda\": 200, \"sph\": {\"SC1\": 100 ,\"SC2\": 100,\"SC3\": 100,\"SC4\": 100,\"SC5\": 100}, \"cts\":\"1606791828\", \"uts\": \"1606791828\"}"]}' -C movieTheatre

peer chaincode invoke -n moviecc -c '{"args":["exs","{\"thid\":\"Theatre1\", \"inventoryid\": \"ES13\", \"cts\":\"1606791828\", \"uts\": \"1606791828\"}"]}' -C movieTheatre

peer chaincode invoke -n moviecc -c '{"args":["sell","{\"thid\": \"Theatre1\", \"moviename\":\"Lucy\", \"screen\":\"SC1\", \"showcode\":\"2\", \"ticketsold\": 3, \"cts\":\"1606791828\", \"uts\": \"1606791828\"}"]}' -C movieTheatre

***********************************************************************************************************/

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

// ShowDetails maintains the show related details.
type ShowDetails struct {
	ObjType   string    `json:"obj"`
	MovieName string    `json:"moviename"`
	Screen    string    `json:"screen"` // Alphanumeric
	TheatreID string    `json:"thid"`   // Alphanumeric
	ShowCode  [4]string `json:"showcode"`
	CreateTs  string    `json:"cts"` // epoch format
	UpdateTs  string    `json:"uts"` // epoch format
}

// TheatreDetails has movie hall-wise max capacity and inventory capacity details
type TheatreDetails struct {
	ObjType       string           `json:"obj"`
	TheatreID     string           `json:"thid"`    // Alphanumeric. Unique for each theatre
	SeatsPerHall  map[string]uint8 `json:"sph"`     // (Max seats count can be set by the theatre. Not hardcoded as 100(as per instruction) to keep it configurable)
	MaxSodaPerDay uint8            `json:"maxsoda"` // Min 0, Max - Count can be set by the theatre. Not hardcoded as 200(as per instruction) to keep it configurable
	CreateTs      string           `json:"cts"`     // epoch format
	UpdateTs      string           `json:"uts"`     // epoch format
}

// SodaInventory keeps track of day-wise soda sale.
type SodaInventory struct {
	ObjType     string `json:"obj"`
	TheatreID   string `json:"thid"`        // Alphanumeric
	InventoryID string `json:"inventoryid"` // Alphanumeric
	SodaSold    uint8  `json:"soda"`        // Min 0, Max - Count set by the theatre in "TheatreDetails" struct.
	CreateTs    string `json:"cts"`         // epoch format
	UpdateTs    string `json:"uts"`         // epoch format
}

// Tickets is the show-wise state data.
type Tickets struct {
	ObjType     string `json:"obj"`
	TheatreID   string `json:"thid"`       // Alphanumeric
	MovieName   string `json:"moviename"`  //
	Screen      string `json:"screen"`     // Alphanumeric
	ShowCode    string `json:"showcode"`   //
	TicketsSold uint8  `json:"ticketsold"` //  Min 0, Max - Count set by the theatre in "TheatreDetails" struct.
	PopCornSold uint8  `json:"pcsold"`     //  Min 0, Max - Equals tickets TicketsSold
	WaterSold   uint8  `json:"watersold"`  //  Min 0, Max - Equals tickets TicketsSold
	CreateTs    string `json:"cts"`        // epoch format
	UpdateTs    string `json:"uts"`        // epoch format
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
		return s.addOrModifyShowDetails(stub, args)
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
func (s *ShowsManagement) addOrModifyShowDetails(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if len(args) != 1 {
		_logger.Info("addOrModifyShowDetails: Incorrect number of arguments provided for the transaction.")
		jsonResp = "{\"Data\":" + strconv.Itoa(len(args)) + ",\"ErrorDetails\":\"Invalid Number of argumnets provided for transaction\"}"
		return shim.Error(jsonResp)
	}

	var sd ShowDetails
	err := json.Unmarshal([]byte(args[0]), &sd)
	if err != nil {
		errorKey = args[0]
		errorData = "Invalid json provided as input"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addOrModifyShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	sc := sd.Screen
	thid := sd.TheatreID

	/* Check if the theatre details is added for the movie hall. If not present, do not process the request */
	theatreDetails, err := stub.GetState(thid)

	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addOrModifyShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if theatreDetails == nil {
		errorData = "Theatre details does not exists for :" + string(thid)
		jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addOrModifyShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	compositeKey := thid + sc // Form composite key with TheatreID and ScreenCode
	// Check if the movie-hall/screen details is present with the theatre
	screenExists, err := stub.GetState(compositeKey)

	if err != nil {
		errorKey = thid
		replaceErr := strings.Replace(err.Error(), "\"", " ", -1)
		errorData = "GetState is Failed :" + replaceErr
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("addOrModifyShowDetails:" + string(jsonResp))
		return shim.Error(jsonResp)
	}
	if screenExists != nil {

		screendetail := ShowDetails{}
		err = json.Unmarshal(screenExists, &screendetail)
		if err != nil {
			errorData = "Existing show details Unmarshalling error"
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
			_logger.Error("addOrModifyShowDetails:" + string(jsonResp))
			return shim.Error(jsonResp)
		}

		sd.ObjType = "ShowDetails"
		sd.UpdateTs = screendetail.UpdateTs
		sdjson, _ := json.Marshal(sd)
		err = stub.PutState(compositeKey, sdjson)
		if err != nil {
			_logger.Errorf("addOrModifyShowDetails:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to add the show details\"}"
			return shim.Error(jsonResp)
		}
		_logger.Infof("addOrModifyShowDetails:Show details added succesfully for theatre :" + string(thid))

	} else {

		// Validate the screen detail against the respective theatre details
		td := TheatreDetails{}
		err = json.Unmarshal(theatreDetails, &td)
		if err != nil {
			errorData = "Existing theatre details Unmarshalling error"
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"" + errorData + "\"}"
			_logger.Error("addOrModifyShowDetails:" + string(jsonResp))
			return shim.Error(jsonResp)
		}
		if td.SeatsPerHall[sc] == 0 {
			errorData = "This screen details does not exists for the theatre"
			jsonResp = "{\"Data\":" + sc + ",\"ErrorDetails\":\"" + errorData + "\"}"
			_logger.Error("addOrModifyShowDetails:" + string(jsonResp))
			return shim.Error(jsonResp)
		}

		sd.ObjType = "ShowDetails"
		sdjson, _ := json.Marshal(sd)
		err = stub.PutState(compositeKey, sdjson)
		if err != nil {
			_logger.Errorf("addOrModifyShowDetails:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to add the show details\"}"
			return shim.Error(jsonResp)
		}
		_logger.Infof("addOrModifyShowDetails:Show details added succesfully for theatre :" + string(thid))
	}
	result := map[string]interface{}{
		"trxnid":  stub.GetTxID(),
		"message": "Add Show Detail Success",
	}
	respjson, _ := json.Marshal(result)
	return shim.Success(respjson)
}

// Get show details on a particular screen of a movie theatre
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

// Add theatre details
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

	thid := td.TheatreID
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
	tdjson, _ := json.Marshal(td)
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

// Sell tickets. 1 popcorn and 1 water bottle issued per ticket
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

	thid := tkt.TheatreID
	sc := tkt.Screen
	st := tkt.ShowCode

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

	// Validation for ticket count
	if tkt.TicketsSold == 0 {
		errorKey = string(tkt.TicketsSold)
		errorData = "Invalid request to sell tickets. Expected 1 or more ticket count"
		jsonResp = "{\"Data\":" + errorKey + ",\"ErrorDetails\":\"" + errorData + "\"}"
		_logger.Error("sellTicket:" + string(jsonResp))
		return shim.Error(jsonResp)
	}

	// Check if tickets sales already started for any given showcode of particular movie-hall
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

	if err != nil {
		_logger.Errorf("sellTicket: String to integer converstion failed ")
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to sell the ticket\"}"
		return shim.Error(jsonResp)
	}

	if err != nil {
		_logger.Errorf("sellTicket: String to integer converstion failed ")
		jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to sell the ticket\"}"
		return shim.Error(jsonResp)
	}

	if tktIssueStarted == nil {

		if tkt.TicketsSold > td.SeatsPerHall[sc] {
			_logger.Error("sellTicket: Enough tickets not available")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Enough tickets not available\"}"
			return shim.Error(jsonResp)
		}

		tkt.ObjType = "Tickets"
		tkt.WaterSold, tkt.PopCornSold = tkt.TicketsSold, tkt.TicketsSold
		tktjson, _ := json.Marshal(tkt)
		err = stub.PutState(compositeKey, tktjson)
		if err != nil {
			_logger.Errorf("sellTicket:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to sell the ticket\"}"
			return shim.Error(jsonResp)
		}
		_logger.Infof("sellTicket:Tickets ticket.TicketsSold successfully")
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
		tkt.UpdateTs = ticket.UpdateTs

		if err != nil {
			_logger.Errorf("sellTicket: String to integer converstion failed ")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Unable to sell the ticket\"}"
			return shim.Error(jsonResp)
		}

		if (ticket.TicketsSold + tkt.TicketsSold) > td.SeatsPerHall[sc] {
			_logger.Error("sellTicket: Enough tickets not available")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\"Enough tickets not available\"}"
			return shim.Error(jsonResp)
		}

		tkt.TicketsSold = ticket.TicketsSold + tkt.TicketsSold
		tkt.WaterSold, tkt.PopCornSold = tkt.TicketsSold, tkt.TicketsSold

		updatedTkt, _ := json.Marshal(tkt)
		err = stub.PutState(compositeKey, updatedTkt)
		if err != nil {
			_logger.Errorf("sellTicket:PutState is Failed :" + string(err.Error()))
			jsonResp = "{\"Data\":" + thid + ",\"ErrorDetails\":\"Unable to sell the ticket\"}"
			return shim.Error(jsonResp)
		}

		_logger.Infof("sellTicket:Tickets ticket.TicketsSold successfully")
	}

	result := map[string]interface{}{
		"trxnid":     stub.GetTxID(),
		"ticketSold": tkt.TicketsSold,
		"message":    "Sell ticket successfull",
	}
	respjson, _ := json.Marshal(result)
	return shim.Success(respjson)

}

// Exchange water with soda
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
	thid := soda.TheatreID
	invid := soda.InventoryID

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
		soda.ObjType = "SodaInventory"
		soda.SodaSold++
		sodajson, _ := json.Marshal(soda)
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
		soda.UpdateTs = sodainv.UpdateTs

		if (sodainv.SodaSold + 1) > td.MaxSodaPerDay {
			_logger.Error("exchangeSoda: Enough soda not available")
			jsonResp = "{\"Data\":\"\",\"ErrorDetails\":\" Enough soda not available\"}"
			return shim.Error(jsonResp)
		}
		soda.SodaSold = sodainv.SodaSold + 1
		updatedinv, _ := json.Marshal(soda)
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

// Generates random number - Decides if the customer is lucky enough to exchange water with soda :)
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
