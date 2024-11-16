package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

var (
	errValidatingZipcodeRequest = "Method not allowed"
	errReadingZipcodeRequest    = "Error reading request body"
	errValidatingZipcode        = "Invalid zipcode"
	errFetchingWeatherRequest   = "Error forwarding weather request"
	errParsingWeatherResponse   = "Error parsing weather response"
	errReturningWeatherResponse = "Error returning weather response"
)

type ZipcodeRequest struct {
	Zipcode string `json:"zipcode"`
}

type WeatherResponse struct {
	City       string  `json:"city"`
	Celsius    float64 `json:"temp_c"`
	Fahrenheit float64 `json:"temp_f"`
	Kelvin     float64 `json:"temp_k"`
}

func handleZipcodeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, errValidatingZipcodeRequest, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, errReadingZipcodeRequest, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var zipcodeRequest ZipcodeRequest
	err = json.Unmarshal(body, &zipcodeRequest)
	if err != nil {
		http.Error(w, errValidatingZipcode, http.StatusUnprocessableEntity)
		return
	}

	if len(zipcodeRequest.Zipcode) != 8 {
		http.Error(w, errValidatingZipcode, http.StatusUnprocessableEntity)
		return
	}

	req, err := http.NewRequest("GET", "http://localhost:8181?zipcode="+zipcodeRequest.Zipcode, bytes.NewBuffer(body))
	if err != nil {
		http.Error(w, errFetchingWeatherRequest, http.StatusInternalServerError)
		return
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, errFetchingWeatherRequest, http.StatusInternalServerError)
		return
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, errParsingWeatherResponse, http.StatusInternalServerError)
		return
	}

	var weatherResponse WeatherResponse
	err = json.Unmarshal(body, &weatherResponse)
	if err != nil {
		http.Error(w, errParsingWeatherResponse+": "+err.Error()+string(body), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(weatherResponse)
	if err != nil {
		http.Error(w, errReturningWeatherResponse, http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/", handleZipcodeRequest)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
