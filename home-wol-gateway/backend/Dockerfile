# syntax=docker/dockerfile:1

FROM golang:1.25.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /home-wol-gateway .

FROM alpine:3.22

RUN apk add --no-cache iproute2

WORKDIR /app

COPY --from=builder /home-wol-gateway /usr/local/bin/home-wol-gateway

EXPOSE 8080
EXPOSE 9090/udp

ENTRYPOINT ["home-wol-gateway"]
