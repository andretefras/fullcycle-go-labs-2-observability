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
	errRequestingZipcode  = "Error requesting zipcode"
	errCannotFindZipcode  = "Can not find zipcode"
)

type ZipcodeRequest struct {
	Zipcode string `json:"zipcode"`
}

type ZipcodeResponse struct {
	Localidade string `json:"localidade"`
}

type TemperaturesResponse struct {
	City       string  `json:"city"`
	Celsius    float64 `json:"temp_c"`
	Fahrenheit float64 `json:"temp_f"`
	Kelvin     float64 `json:"temp_k"`
}

func handleZipcodeRequest(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, errInvalidZipcode, http.StatusUnprocessableEntity)
		return
	}

	if len(zipcodeRequest.Zipcode) != 8 {
		http.Error(w, errInvalidZipcode, http.StatusUnprocessableEntity)
		return
	}

	req, err := http.NewRequest("POST", "viacep.com.br/ws/"+zipcodeRequest.Zipcode+"/json/", nil)
	if err != nil {
		http.Error(w, errRequestingZipcode, http.StatusUnprocessableEntity)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, errCannotFindZipcode, http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, errRequestingZipcode, http.StatusInternalServerError)
		return
	}

	var zipcodeResponse ZipcodeResponse
	err = json.Unmarshal(body, &zipcodeResponse)
	if err != nil {
		http.Error(w, errRequestingZipcode, http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/", handleZipcodeRequest)
	log.Fatal(http.ListenAndServe(":8181", nil))
}
