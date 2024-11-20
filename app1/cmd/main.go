package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/metric"
	"io"
	"net/http"
	"time"
)

var (
	errValidatingZipcodeRequest = "Method not allowed"
	errReadingZipcodeRequest    = "Error reading request body"
	errValidatingZipcode        = "Invalid zipcode"
	errFetchingWeatherRequest   = "Error forwarding weather request"
	errParsingWeatherResponse   = "Error parsing weather response"
	errReturningWeatherResponse = "Error returning weather response"
)

const name = "go.opentelemetry.io/otel/example/dice"

var (
	tracer     = otel.Tracer(name)
	meter      = otel.Meter(name)
	logger     = otelslog.NewLogger(name)
	requestCnt metric.Int64Counter
)

func init() {
	var err error
	requestCnt, err = meter.Int64Counter("cep.requests",
		metric.WithDescription("The number of cep requests"),
		metric.WithUnit("{request}"))
	if err != nil {
		panic(err)
	}
}

type ZipcodeRequest struct {
	Zipcode string `json:"zipcode"`
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

	tracer := otel.Tracer("app1-tracer")
	meter := otel.Meter("app1-meter")

	method, _ := baggage.NewMember("method", "post")
	client, _ := baggage.NewMember("client", "http")
	bag, _ := baggage.New(method, client)

	// labels represent additional key-value descriptors that can be bound to a
	// metric observer or recorder.
	// TODO: Use baggage when supported to extract labels from baggage.
	commonLabels := []attribute.KeyValue{
		attribute.String("method", "post"),
		attribute.String("client", "http"),
	}

	// Recorder metric example
	requestLatency, _ := meter.Float64Histogram(
		"app1/request_latency",
		metric.WithDescription("The latency of requests processed"),
	)

	// TODO: Use a view to just count number of measurements for requestLatency when available.
	requestCount, _ := meter.Int64Counter(
		"app1/request_counts",
		metric.WithDescription("The number of requests processed"),
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		defaultCtx := baggage.ContextWithBaggage(context.Background(), bag)

		startTime := time.Now()
		ctx, span := tracer.Start(defaultCtx, "Check Weather")

		requestCount.Add(ctx, 1, metric.WithAttributes(commonLabels...))

		span.AddEvent("Validating request")

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

		span.AddEvent("Making request")
		req, err := http.NewRequestWithContext(ctx, "GET", "http://app2:8181?zipcode="+zipcodeRequest.Zipcode, bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, errFetchingWeatherRequest, http.StatusInternalServerError)
			return
		}

		httpClient := http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			http.Error(w, errFetchingWeatherRequest, http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		span.AddEvent("Reading response")

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

		span.End()
		latencyMs := float64(time.Since(startTime)) / 1e6

		requestLatency.Record(ctx, latencyMs, metric.WithAttributes(commonLabels...))

		fmt.Printf("Latency: %.3fms\n", latencyMs)
	})

	err := http.ListenAndServe(":8080", handler)
	if err != nil {
		handleErr(err, "server failed to serve")
	}
}
