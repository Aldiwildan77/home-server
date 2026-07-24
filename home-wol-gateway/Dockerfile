# syntax=docker/dockerfile:1
#
# Build from the home-wol-gateway/ root (not backend/) so this stage can
# see frontend/ too:
#   cd home-wol-gateway && docker build -t home-wol-gateway .

FROM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25.1-alpine AS builder
WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
COPY --from=frontend /src/dist/ ./web/dist/

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
