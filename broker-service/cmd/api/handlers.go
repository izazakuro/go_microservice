package main

import (
	"broker/event"
	"broker/logs"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/rpc"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RequestPayload struct {
	Action string      `json:"action"`
	Auth   AuthPayload `json:"auth,omitempty"`
	Log    LogPayload  `json:"log,omitempty"`
	Mail   MailPayload `json:"mail,omitempty"`
}

type MailPayload struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type AuthPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LogPayload struct {
	Name string `json:"name"`
	Data string `json:"data"`
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
	case "log":
		app.logItemViaRPC(w, requestPayload.Log)
	case "mail":
		app.sendMail(w, requestPayload.Mail)
	default:
		app.errJSON(w, errors.New("unknown action"))
	}
}

func (app *Config) logItem(w http.ResponseWriter, entry LogPayload) {
	jsonData, _ := json.MarshalIndent(entry, "", "\t")

	logServiceURL := "http://logger-service/log"

	request, err := http.NewRequest("POST", logServiceURL, bytes.NewBuffer(jsonData))

	if err != nil {
		app.errJSON(w, err)
		return
	}

	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		app.errJSON(w, err)
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		app.errJSON(w, err)
		return
	}

	var payload jsonResponse
	payload.Error = false
	payload.Message = "logged"

	app.writeJSON(w, http.StatusAccepted, payload)

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

func (app *Config) sendMail(w http.ResponseWriter, msg MailPayload) {
	jsonData, _ := json.MarshalIndent(msg, "", "\t")

	// call the mail service
	mailServiceURL := "http://mailer-service/send"

	// Post to mail service
	request, err := http.NewRequest("POST", mailServiceURL, bytes.NewBuffer(jsonData))
	if err != nil {
		app.errJSON(w, err)
		return
	}

	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		app.errJSON(w, err)
		return
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		app.errJSON(w, errors.New("error calling mail service"))
		return
	}

	// send back json
	var payload jsonResponse

	payload.Error = false
	payload.Message = "Message sent to" + msg.To

	app.writeJSON(w, http.StatusAccepted, payload)

}
func (app *Config) logEventViaRabbit(w http.ResponseWriter, l LogPayload) {
	err := app.pushToQueue(l.Name, l.Data)
	if err != nil {
		app.errJSON(w, err)
		return
	}

	var payload jsonResponse
	payload.Error = false
	payload.Message = "logged via RabbitMQ"

	app.writeJSON(w, http.StatusAccepted, payload)
}

func (app *Config) pushToQueue(name, msg string) error {
	emitter, err := event.NewEventEmitter(app.Rabbit)

	if err != nil {
		return err
	}

	payload := LogPayload{
		Name: name,
		Data: msg,
	}

	j, _ := json.MarshalIndent(&payload, "", "\t")
	err = emitter.Push(string(j), "log.INFO")
	if err != nil {
		return err
	}

	return nil
}

type RpcPayload struct {
	Name string
	Data string
}

func (app *Config) logItemViaRPC(w http.ResponseWriter, l LogPayload) {
	client, err := rpc.Dial("tcp", "logger-service:5001")
	if err != nil {
		app.errJSON(w, err)
		return
	}

	rpcPayload := RpcPayload{
		Name: l.Name,
		Data: l.Data,
	}

	var result string

	err = client.Call("RPCServer.LogInfo", rpcPayload, &result)

	if err != nil {
		app.errJSON(w, err)
		return
	}

	payload := jsonResponse{
		Error:   false,
		Message: result,
	}

	app.writeJSON(w, http.StatusAccepted, payload)

}

func (app *Config) LogViaGRPC(w http.ResponseWriter, r *http.Request) {
	var requestPayload RequestPayload

	err := app.readJSON(w, r, &requestPayload)
	if err != nil {
		app.errJSON(w, err)
		return
	}

	conn, err := grpc.NewClient("logger-service:50001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		app.errJSON(w, err)
		return
	}

	defer conn.Close()

	c := logs.NewLogServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = c.WriteLog(ctx, &logs.LogRequest{
		LogEntry: &logs.Log{
			Name: requestPayload.Log.Name,
			Data: requestPayload.Log.Data,
		},
	})
	if err != nil {
		app.errJSON(w, err)
		return
	}

	var payload jsonResponse
	payload.Error = false
	payload.Message = "logged"

	app.writeJSON(w, http.StatusAccepted, payload)

}
