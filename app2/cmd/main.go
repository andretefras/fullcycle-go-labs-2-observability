package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	errValidatingRequestMethod  = "Method not allowed"
	errReadingRequestBody       = "Error reading request body"
	errValidatingRequestZipcode = "Invalid zipcode"
	errRequestingZipcode        = "Error requesting zipcode"
	errFindingZipcode           = "Can not find zipcode"
	errParsingZipcode           = "Error parsing zipcode"
	errRequestingWeather        = "Error requesting weather"
	errMissingApiKey            = "Error finding weather api key"
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

func main() {
	shutdown := initProvider()
	defer shutdown()

	meter := otel.Meter("app2-meter")
	serverAttribute := attribute.String("server-attribute", "foo")
	commonLabels := []attribute.KeyValue{serverAttribute}
	requestCount, _ := meter.Int64Counter(
		"app2/request_counts",
		metric.WithDescription("The number of requests received"),
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestCount.Add(ctx, 1, metric.WithAttributes(commonLabels...))
		span := trace.SpanFromContext(ctx)
		bag := baggage.FromContext(ctx)

		var baggageAttributes []attribute.KeyValue
		baggageAttributes = append(baggageAttributes, serverAttribute)
		for _, member := range bag.Members() {
			baggageAttributes = append(baggageAttributes, attribute.String("baggage key:"+member.Key(), member.Value()))
		}
		span.SetAttributes(baggageAttributes...)

		span.AddEvent("Validating request")

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

		span.AddEvent("Validating zipcode")
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

		span.AddEvent("Getting zipcode data")
		zipcodeResponse, err := fetchZipcodeApi(ctx, w, zipcodeRequest)
		if err != nil {
			return
		}

		span.AddEvent("Getting weather data")
		weatherResponse, err := fetchWeatherApi(ctx, w, zipcodeResponse)
		if err != nil {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(weatherResponse)
		if err != nil {
			http.Error(w, errRequestingWeather, http.StatusInternalServerError)
			return
		}
	})

	mux := http.NewServeMux()
	mux.Handle("/", otelhttp.NewHandler(handler, "Receive request"))
	server := &http.Server{
		Addr:        ":8181",
		Handler:     mux,
		ReadTimeout: 20 * time.Second,
	}
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		handleErr(err, "server failed to serve")
	}
}

func fetchZipcodeApi(ctx context.Context, w http.ResponseWriter, zipcodeRequest ZipcodeRequest) (ZipcodeResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://viacep.com.br/ws/"+zipcodeRequest.Zipcode+"/json/", nil)
	if err != nil {
		http.Error(w, errRequestingZipcode, http.StatusUnprocessableEntity)
		return ZipcodeResponse{}, errors.New(errRequestingZipcode)
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
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

func fetchWeatherApi(ctx context.Context, w http.ResponseWriter, zipcodeResponse ZipcodeResponse) (WeatherResponse, error) {
	params := url.Values{}
	params.Add("q", zipcodeResponse.Localidade)
	fullUrl := fmt.Sprintf("%s?%s", "https://api.weatherapi.com/v1/current.json", params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", fullUrl, nil)
	if err != nil {
		http.Error(w, errRequestingWeather, http.StatusUnprocessableEntity)
		return WeatherResponse{}, errors.New(errRequestingWeather)
	}

	weatherApiKey, ok := os.LookupEnv("WEATHER_API_KEY")
	if !ok {
		http.Error(w, errMissingApiKey, http.StatusUnprocessableEntity)
		return WeatherResponse{}, errors.New(errMissingApiKey)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("key", weatherApiKey)
	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
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
