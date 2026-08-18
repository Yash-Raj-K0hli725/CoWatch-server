# syntax=docker/dockerfile:1

# --- Build ---------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/server

# --- Runtime ---------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api

EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/api"]
