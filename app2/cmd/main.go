package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

var (
	errValidatingRequestMethod  = "Method not allowed"
	errReadingRequestBody       = "Error reading request body"
	errValidatingRequestZipcode = "Invalid zipcode"
	errRequestingZipcode        = "Error requesting zipcode"
	errFindingZipcode           = "Can not find zipcode"
	errParsingZipcode           = "Error parsing zipcode"
	errRequestingWeather        = "Error requesting weather"
	errParsingWeather           = "Error parsing weather"
)

type ZipcodeRequest struct {
	Zipcode string `json:"zipcode"`
}

type ZipcodeResponse struct {
	Localidade string `json:"localidade"`
	Erro       string `json:"erro"`
}

type WeatherResponse struct {
	City       string  `json:"city"`
	Celsius    float64 `json:"temp_c"`
	Fahrenheit float64 `json:"temp_f"`
	Kelvin     float64 `json:"temp_k"`
}

func handleZipcodeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, errValidatingRequestMethod, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("%s\n", string(body))
		http.Error(w, errReadingRequestBody, http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var zipcodeRequest ZipcodeRequest
	err = json.Unmarshal(body, &zipcodeRequest)
	if err != nil {
		http.Error(w, errValidatingRequestZipcode, http.StatusUnprocessableEntity)
		return
	}

	if len(zipcodeRequest.Zipcode) != 8 {
		http.Error(w, errValidatingRequestZipcode, http.StatusUnprocessableEntity)
		return
	}

	zipcodeResponse, err := fetchZipcode(w, zipcodeRequest)
	if err != nil {
		return
	}

	weatherResponse, err := fetchWeatherApi(w, zipcodeResponse)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(weatherResponse)
	if err != nil {
		http.Error(w, errRequestingWeather, http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/", handleZipcodeRequest)
	log.Fatal(http.ListenAndServe(":8181", nil))
}

func fetchZipcode(w http.ResponseWriter, zipcodeRequest ZipcodeRequest) (ZipcodeResponse, error) {
	fmt.Printf("%s\n", zipcodeRequest)
	req, err := http.NewRequest("GET", "https://viacep.com.br/ws/"+zipcodeRequest.Zipcode+"/json/", nil)
	if err != nil {
		http.Error(w, errRequestingZipcode, http.StatusUnprocessableEntity)
		return ZipcodeResponse{}, errors.New(errRequestingZipcode)
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, errRequestingZipcode, http.StatusNotFound)
		return ZipcodeResponse{}, errors.New(errFindingZipcode)
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		http.Error(w, errFindingZipcode, http.StatusInternalServerError)
		return ZipcodeResponse{}, errors.New(errRequestingWeather)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, errParsingZipcode, http.StatusInternalServerError)
		return ZipcodeResponse{}, errors.New(errRequestingZipcode)
	}
	fmt.Printf("%s\n", string(body))

	var zipcodeResponse ZipcodeResponse
	err = json.Unmarshal(body, &zipcodeResponse)
	if err != nil {
		http.Error(w, errParsingZipcode, http.StatusInternalServerError)
		return ZipcodeResponse{}, errors.New(errParsingZipcode)
	}

	if zipcodeResponse.Erro != "" {
		http.Error(w, errFindingZipcode, http.StatusNotFound)
		return ZipcodeResponse{}, errors.New(errFindingZipcode)
	}

	return zipcodeResponse, nil
}

func fetchWeatherApi(w http.ResponseWriter, zipcodeResponse ZipcodeResponse) (WeatherResponse, error) {
	fmt.Printf("%s\n", zipcodeResponse)
	params := url.Values{}
	params.Add("q", zipcodeResponse.Localidade)
	fullUrl := fmt.Sprintf("%s?%s", "https://api.weatherapi.com/v1/current.json", params.Encode())
	req, err := http.NewRequest("GET", fullUrl, nil)
	if err != nil {
		http.Error(w, errRequestingWeather, http.StatusUnprocessableEntity)
		return WeatherResponse{}, errors.New(errRequestingWeather)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("key", "b88fdf1171c0464d908220432241511")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, errRequestingWeather, http.StatusInternalServerError)
		return WeatherResponse{}, errors.New(errRequestingWeather)
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		http.Error(w, errRequestingWeather, http.StatusInternalServerError)
		return WeatherResponse{}, errors.New(errRequestingWeather)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, errRequestingWeather, http.StatusInternalServerError)
		return WeatherResponse{}, errors.New(errRequestingWeather)
	}
	fmt.Printf("%s\n", string(body))

	var weatherApiResponse map[string]interface{}
	err = json.Unmarshal(body, &weatherApiResponse)
	if err != nil {
		http.Error(w, errParsingWeather, http.StatusInternalServerError)
		return WeatherResponse{}, errors.New(errParsingWeather)
	}

	var weatherResponse WeatherResponse
	weatherResponse.City = weatherApiResponse["location"].(map[string]interface{})["name"].(string)
	weatherResponse.Celsius = weatherApiResponse["current"].(map[string]interface{})["temp_c"].(float64)
	weatherResponse.Fahrenheit = weatherApiResponse["current"].(map[string]interface{})["temp_f"].(float64)
	weatherResponse.Kelvin = weatherResponse.Celsius + 273

	return weatherResponse, nil
}
