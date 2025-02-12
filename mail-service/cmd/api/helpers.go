package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type jsonResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (app *Config) readJSON(w http.ResponseWriter, req *http.Request, data interface{}) error {
	maxBytes := 1048576 // 1 mb

	req.Body = http.MaxBytesReader(w, req.Body, int64(maxBytes))

	dec := json.NewDecoder(req.Body)
	err := dec.Decode(&data)
	if err != nil {
		return err
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("Body must have only a single JSON value")
	}

	return nil
}

func (app *Config) writeJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	m, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(status)
	_, err = w.Write(m)

	if err != nil {
		return err
	}

	return nil
}

func (app *Config) errJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload jsonResponse
	payload.Error = true
	payload.Message = err.Error()

	return app.writeJSON(w, statusCode, payload)
}
