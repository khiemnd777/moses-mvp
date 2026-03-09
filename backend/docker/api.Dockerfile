FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api

FROM alpine:3.19
WORKDIR /app
COPY --from=build /bin/api /app/api
COPY config /app/config
EXPOSE 8080
ENV CONFIG_PATH=/app/config/config.yaml
CMD ["/app/api"]
