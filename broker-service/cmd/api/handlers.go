package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type RequestPayload struct {
	Action string      `json:"action"`
	Auth   AuthPayload `json:"auth,omitempty"`
}

type AuthPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (app *Config) Broker(w http.ResponseWriter, req *http.Request) {
	payload := jsonResponse{
		Error:   false,
		Message: "Hit the broker",
	}

	_ = app.writeJSON(w, http.StatusOK, payload)
}

func (app *Config) HandleSubmission(w http.ResponseWriter, req *http.Request) {
	var requestPayload RequestPayload

	err := app.readJSON(w, req, &requestPayload)
	if err != nil {
		app.errJSON(w, err)
		return
	}

	switch requestPayload.Action {
	case "auth":
		app.authenticate(w, requestPayload.Auth)
	default:
		app.errJSON(w, errors.New("unknown action"))
	}
}

func (app *Config) authenticate(w http.ResponseWriter, a AuthPayload) {
	//create json and send to auth microservice
	m, _ := json.MarshalIndent(a, "", "\t")
	// call the service
	req, err := http.NewRequest("POST", "http://authentication-service/authenticate", bytes.NewBuffer(m))
	if err != nil {
		app.errJSON(w, err)
		return
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		app.errJSON(w, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		app.errJSON(w, errors.New("Invalid Credential"))
		return
	} else if resp.StatusCode != http.StatusAccepted {
		log.Printf("Status Code: %v", resp.StatusCode)
		app.errJSON(w, errors.New("Error calling auth"))
		return
	}

	// create var read response body
	var jsonResp jsonResponse

	// decode json from other service
	err = json.NewDecoder(resp.Body).Decode(&jsonResp)
	if err != nil {
		app.errJSON(w, err)
		return
	}

	if jsonResp.Error {
		app.errJSON(w, err, http.StatusUnauthorized)
		return
	}

	payload := jsonResponse{
		Error:   false,
		Message: "Authenticated!",
		Data:    jsonResp.Data,
	}

	app.writeJSON(w, http.StatusAccepted, payload)
}
