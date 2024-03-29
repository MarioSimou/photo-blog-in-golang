package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MarioSimou/authAPI/internal/models"
	"github.com/MarioSimou/authAPI/internal/utils"
	"github.com/MarioSimou/authAPI/internal/utils/httpcodes"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var c *Controller
var users models.Users
var payloads []*utils.Payload
var u utils.Utils

type Check struct {
	Key      string
	Expected interface{}
}

func checkJSON(json map[string]interface{}, checks []Check, t *testing.T) {
	for _, v := range checks {
		k := v.Key
		e := v.Expected

		vv := json[k]

		if vv != e {
			t.Errorf("Should return a %v of %v rather than %v", k, e, vv)
		}
	}
}

func checkStatusCode(res *http.Response, expected int, t *testing.T) {
	if code := res.StatusCode; code != expected {
		t.Errorf("Should return a status code of %d rather than %d", expected, code)
	}
}

func checkHeader(w *httptest.ResponseRecorder, k string, expected string, t *testing.T) {
	if ct := w.Header().Get(k); ct != expected {
		t.Errorf("Should return a %v of %v rather than %v", k, expected, ct)
	}
}

func checkBody(res *http.Response, expected string, t *testing.T) {
	body, _ := ioutil.ReadAll(res.Body)
	if b := strings.TrimRight(string(body), "\n"); b != expected {
		t.Errorf("Should return a body of %v rather than %v", expected, b)
	}
}
func convertResponseToJson(res *http.Response) *httpcodes.Representation {
	var response httpcodes.Representation
	bf, _ := ioutil.ReadAll(res.Body)
	json.Unmarshal(bf, &response)
	return &response
}

func generateUserPayload(user models.User) *utils.Payload {
	token, ok := u.GenerateToken(user, os.Getenv("JWT_SECRET"), time.Hour)
	if !ok {
		log.Fatal("Unable to generate mock JWT token")
	}
	return u.ExtractPayload(string(token))
}
func mockData(mcli *utils.MongoClient) {
	usersCollection := mcli.Client.Database(mcli.Database).Collection("users")
	e := usersCollection.Drop(context.Background())
	if e != nil {
		log.Fatal("Unable to drop Users collection")
	}

	paulID, _ := primitive.ObjectIDFromHex("5db5b5b06507b38887bedc87")
	paul := models.User{Id: &paulID, Username: "paul", Email: "paul@gmail.com", Password: "$2a$04$DzlgE3dAEEynd4Ed9z0oY.MafLBCoZl815bXXeOjekaZztjwDLcdm", Role: "BASIC"}
	johnID, _ := primitive.ObjectIDFromHex("5db5b5b06507b38887bedc88")
	john := models.User{Id: &johnID, Username: "john", Email: "john@gmail.com", Password: "$2a$04$DzlgE3dAEEynd4Ed9z0oY.MafLBCoZl815bXXeOjekaZztjwDLcdm", Role: "BASIC"}

	result, _ := usersCollection.InsertMany(context.Background(), []interface{}{john, paul})
	if result.InsertedIDs == nil {
		log.Fatal("Data has not being loaded to the database")
	}

	// retrieves the users stored in the database
	cur, e := usersCollection.Find(context.TODO(), bson.M{})
	if e != nil {
		log.Fatal("Unable to fetch mocked usrs")
	}
	for cur.Next(context.Background()) {
		var user models.User
		cur.Decode(&user)

		users = append(users, user)
	}
	cur.Close(context.Background())

	// mocks a JWT token
	payloads = []*utils.Payload{
		generateUserPayload(john),
		generateUserPayload(paul),
	}
}

func init() {
	u = utils.Utils{}
	dir, _ := os.Getwd()
	fmt.Println(dir)
	u.LoadDotEnv("../../configs/.test.env")
	mcli := u.ConnectDatabase(os.Getenv("MONGO_URI"), os.Getenv("DB_NAME"))
	c = NewController(mcli, &u)
	mockData(mcli)
}

func TestGetUsers(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/users", nil)
	c.GetUsers(w, r, nil)

	res := w.Result()
	checkStatusCode(res, 200, t)
	checkHeader(w, "Content-Type", "application/json", t)

	body, _ := ioutil.ReadAll(res.Body)
	expected := `{"status":200,"success":true,"message":"Successful fetch","data":[{"username":"john","email":"john@gmail.com","role":"BASIC"},{"username":"paul","email":"paul@gmail.com","role":"BASIC"}]}`

	if b := strings.TrimRight(string(body), "\n"); b != expected {
		t.Errorf("Should return a body of %v rather than %v", expected, b)
	}
}

func TestGetUserPaulFromPaul(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/users/5db5b5b06507b38887bedc87", nil)
	c.GetUser(w, r, httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedc87"}}, payloads[1])

	res := w.Result()
	checkStatusCode(res, 200, t)
	checkHeader(w, "Content-Type", "application/json", t)

	response := convertResponseToJson(res)
	if response.Status != 200 {
		t.Errorf("Should return a status of %v rather than %v", 200, response.Status)
	}
	if !response.Success {
		t.Errorf("Should return a success of %v rather than %v", true, response.Success)
	}
	user := response.Data.(map[string]interface{})
	checkJSON(user, []Check{
		Check{Key: "id", Expected: "5db5b5b06507b38887bedc87"},
		Check{Key: "username", Expected: "paul"},
		Check{Key: "email", Expected: "paul@gmail.com"},
		Check{Key: "role", Expected: "BASIC"},
		Check{Key: "password", Expected: "$2a$04$DzlgE3dAEEynd4Ed9z0oY.MafLBCoZl815bXXeOjekaZztjwDLcdm"},
	}, t)
}

func TestGetUserPaulFromJohn(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/users/5db5b5b06507b38887bedc87", nil)
	c.GetUser(w, r, httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedc87"}}, payloads[0])

	res := w.Result()
	checkStatusCode(res, 200, t)
	checkHeader(w, "Content-Type", "application/json", t)
	response := convertResponseToJson(res)
	if response.Status != 200 {
		t.Errorf("Should return a status of %v rather than %v", 200, response.Status)
	}
	if !response.Success {
		t.Errorf("Should return a success of %v rather than %v", true, response.Success)
	}
	user := response.Data.(map[string]interface{})
	checkJSON(user, []Check{
		Check{Key: "id", Expected: "5db5b5b06507b38887bedc87"},
		Check{Key: "password", Expected: nil},
		Check{Key: "username", Expected: "paul"},
		Check{Key: "email", Expected: "paul@gmail.com"},
		Check{Key: "role", Expected: "BASIC"},
	}, t)
}

func TestGetUnknownUser(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/users/5db5b5b06507b38887bedaa91", nil)
	c.GetUser(w, r, httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedaa91"}}, payloads[0])

	res := w.Result()
	checkStatusCode(res, 404, t)
	checkHeader(w, "Content-Type", "application/json", t)
	response := convertResponseToJson(res)
	if response.Status != 404 {
		t.Errorf("Should return a status of %v rather than %v", 404, response.Status)
	}
	if response.Success {
		t.Errorf("Should return a success of %v rather than %v", false, response.Success)
	}
}

func TestInsertOne(t *testing.T) {
	w := httptest.NewRecorder()
	bf := []byte(`{"id":"5db5a5e53b0bb99f8ef32116","username":"michael","password":"12345678","email":"michael@gmail.com","role":"BASIC"}`)
	body := ioutil.NopCloser(bytes.NewBuffer(bf))
	r := httptest.NewRequest("POST", "/api/v1/users", body)
	c.CreateUser(w, r, nil)

	res := w.Result()
	checkStatusCode(res, 201, t)
	checkHeader(w, "Content-Type", "application/json", t)
	response := convertResponseToJson(res)

	if response.Status != 201 {
		t.Errorf("Should return a status of %v rather than %v", 201, response.Status)
	}
	if !response.Success {
		t.Errorf("Should return a success of %v rather than %v", true, response.Success)
	}

	user := response.Data.(map[string]interface{})
	checkJSON(user, []Check{
		Check{Key: "id", Expected: "5db5a5e53b0bb99f8ef32116"},
		Check{Key: "password", Expected: "12345678"},
		Check{Key: "username", Expected: "michael"},
		Check{Key: "email", Expected: "michael@gmail.com"},
		Check{Key: "role", Expected: "BASIC"},
	}, t)

	expectedLocationHeader := "/api/v1/users/5db5a5e53b0bb99f8ef32116"
	if loc := w.Header().Get("Location"); loc != expectedLocationHeader {
		t.Errorf("Should return a success of %v rather than %v", expectedLocationHeader, loc)
	}
}

func TestUpdateUserPaulFromPaul(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"username":"paul37","email":"mrpaul@gmail.com"}`)
	r := httptest.NewRequest("PUT", "/api/v1/users/5db5b5b06507b38887bedc87", bytes.NewBuffer(body))
	params := httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedc87"}}
	c.UpdateUser(w, r, params, payloads[1])

	res := w.Result()
	checkStatusCode(res, 200, t)
	checkHeader(w, "Content-Type", "application/json", t)
	response := convertResponseToJson(res)

	if response.Status != 200 {
		t.Errorf("Should return a status of %v rather than %v", 200, response.Status)
	}
	if !response.Success {
		t.Errorf("Should return a success of %v rather than %v", true, response.Success)
	}

	user := response.Data.(map[string]interface{})
	checkJSON(user, []Check{
		Check{Key: "username", Expected: "paul37"},
		Check{Key: "email", Expected: "mrpaul@gmail.com"},
	}, t)
}

func TestUpdateUserPaulFromJohn(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"username":"paul37","email":"mrpaul@gmail.com"}`)
	r := httptest.NewRequest("PUT", "/api/v1/users/5db5b5b06507b38887bedc87", bytes.NewBuffer(body))
	params := httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedc87"}}
	c.UpdateUser(w, r, params, payloads[0])

	res := w.Result()
	checkStatusCode(res, 403, t)
	response := convertResponseToJson(res)
	if response.Status != 403 {
		t.Errorf("Should return a status code of %v rather than %v", 403, response.Status)
	}
	if response.Success {
		t.Errorf("Should return a success of %v rather than %v", false, response.Success)
	}
	if response.Data != nil {
		t.Errorf("Should not have returned any data")
	}
}

func TestUpdateANoneExistingResource(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"username":"paul37","email":"mrpaul@gmail.com"}`)
	r := httptest.NewRequest("PUT", "/api/v1/users/5db5b5b06507b38887bedc87", bytes.NewBuffer(body))
	params := httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedc843"}}
	c.UpdateUser(w, r, params, payloads[0])

	res := w.Result()
	checkStatusCode(res, 403, t)
	response := convertResponseToJson(res)
	if response.Status != 403 {
		t.Errorf("Should return a status code of %v rather than %v", 403, response.Status)
	}
	if response.Success {
		t.Errorf("Should return a success of %v rather than %v", false, response.Success)
	}
	if response.Data != nil {
		t.Errorf("Should not have returned any data")
	}
}

func TestDeletePaulFromPaul(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/users/5db5b5b06507b38887bedc87", nil)
	c.DeleteUser(w, r, httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedc87"}}, payloads[1])

	res := w.Result()
	checkStatusCode(res, 204, t)
	checkHeader(w, "Content-Type", "application/json", t)
}

func TestDeleteWithoutATargetResource(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/users", nil)
	c.DeleteUser(w, r, nil, payloads[1])

	res := w.Result()
	checkStatusCode(res, 400, t)
	checkHeader(w, "Content-Type", "application/json", t)
	response := convertResponseToJson(res)
	if response.Status != 400 {
		t.Errorf("Should return a status code of 400 rather than %v", response.Status)
	}
	if response.Success {
		t.Errorf("Should retun a success of false rather than %v", response.Success)
	}
}

func TestDeletePaulFromJohn(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/users/5db5b5b06507b38887bedc87", nil)
	c.DeleteUser(w, r, httprouter.Params{httprouter.Param{Key: "id", Value: "5db5b5b06507b38887bedc87"}}, payloads[0])

	res := w.Result()
	checkStatusCode(res, 403, t)
	response := convertResponseToJson(res)
	if response.Status != 403 {
		t.Errorf("Should return a status code of %v rather than %v", 403, response.Status)
	}
	if response.Success {
		t.Errorf("Should return a success of %v rather than %v", false, response.Success)
	}
	if response.Data != nil {
		t.Errorf("Should not have returned any data")
	}
}

func TestSignIn(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"email":"john@gmail.com","password":"12345678"}`)
	r := httptest.NewRequest("POST", "/api/v1/users/signin", bytes.NewBuffer(body)) // bytes to Reader
	c.SignIn(w, r, nil)

	res := w.Result()
	checkStatusCode(res, 200, t)
	checkHeader(w, "Content-Type", "application/json", t)

	response := convertResponseToJson(res)
	if response.Status != 200 {
		t.Errorf("Should return a status of %v rather than %v", 200, response.Status)
	}
	if !response.Success {
		t.Errorf("Should return a success of %v rather than %v", true, response.Success)
	}

	user := response.Data.(map[string]interface{})
	checkJSON(user, []Check{
		Check{Key: "username", Expected: "john"},
		Check{Key: "email", Expected: "john@gmail.com"},
		Check{Key: "role", Expected: "BASIC"},
	}, t)
}

func TestInvalidSignIn(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"email":"unknown@gmail.com","password":"12345678"}`)
	r := httptest.NewRequest("POST", "/api/v1/users/signin", bytes.NewBuffer(body)) // bytes to Reader
	c.SignIn(w, r, nil)

	res := w.Result()
	checkStatusCode(res, 404, t)
	checkHeader(w, "Content-Type", "application/json", t)

	response := convertResponseToJson(res)
	if response.Status != 404 {
		t.Errorf("Should return a status of %v rather than %v", 404, response.Status)
	}
	if response.Success {
		t.Errorf("Should return a success of %v rather than %v", false, response.Success)
	}
}

func TestInvalidPassword(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"email":"john@gmail.com","password":"1234567810"}`)
	r := httptest.NewRequest("POST", "/api/v1/users/signin", bytes.NewBuffer(body)) // bytes to Reader
	c.SignIn(w, r, nil)

	res := w.Result()
	checkStatusCode(res, 401, t)
	checkHeader(w, "Content-Type", "application/json", t)

	response := convertResponseToJson(res)
	if response.Status != 401 {
		t.Errorf("Should return a status of %v rather than %v", 401, response.Status)
	}
	if response.Success {
		t.Errorf("Should return a success of %v rather than %v", false, response.Success)
	}
}
