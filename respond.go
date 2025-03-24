package main

import (
	"net/http"
	"encoding/json"
	"fmt"
)

// sends a string response
func respondWithString(wri http.ResponseWriter, code int, msg string) {
	wri.Header().Set("Content-Type", "text/plain; charset=utf-8")
	wri.WriteHeader(code)
	wri.Write([]byte(msg))
}

// marshals the JSON data and sends it in response
func respondWithJSON(wri http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprintf("Error marshalling response: %v", err))
		return
	}
	wri.Header().Set("Content-Type", "application/json")
	wri.WriteHeader(code)
	wri.Write(dat)
}

// sends an error response
func respondWithError(wri http.ResponseWriter, code int, msg string) {
	type errorResp struct {
		Error string `json:"error"`
	}
	res := errorResp{Error: msg}
	ret, _ := json.Marshal(res)
	wri.Header().Set("Content-Type", "application/json")
	wri.WriteHeader(code)
	wri.Write(ret)
}