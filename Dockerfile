FROM golang:latest as builder
ARG APP=app1
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags="-w -s" -o ${APP} ${APP}/cmd/main.go
FROM alpine:latest
COPY --from=builder /app/${APP} /app/${APP}
CMD ["/app/${APP}"]