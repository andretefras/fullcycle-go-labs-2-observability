package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

var (
	errInvalidZipcode     = "Invalid zipcode"
	errMethodNotAllowed   = "Method not allowed"
	errReadingRequestBody = "Error reading request body"
)

type ZipcodeRequest struct {
	Zipcode string `json:"zipcode"`
}

func handleAddressRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, errReadingRequestBody, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var zipcodeRequest ZipcodeRequest
	err = json.Unmarshal(body, &zipcodeRequest)
	if err != nil {
		http.Error(w, errInvalidZipcode, http.StatusBadRequest)
		return
	}

	if len(zipcodeRequest.Zipcode) != 8 {
		http.Error(w, errInvalidZipcode, http.StatusBadRequest)
		return
	}
}

func main() {
	http.HandleFunc("/", handleAddressRequest)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
