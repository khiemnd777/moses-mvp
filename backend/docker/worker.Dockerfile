FROM golang:1.24-alpine AS build

WORKDIR /app

RUN apk add --no-cache git antiword
COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker


FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates antiword

COPY --from=build /bin/worker /app/worker
COPY config /app/config

CMD ["/app/worker"]
