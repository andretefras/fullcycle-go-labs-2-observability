FROM golang:latest as builder
ARG APP=app1
WORKDIR /app
COPY . .
RUN cd ${APP} && \
    go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags="-w -s" -o ../app ./cmd/main.go ./cmd/otel.go
FROM alpine:latest
COPY --from=builder /app/${APP} /app/${APP}
CMD ["/app/app"]