FROM golang:1.27-alpine AS build

WORKDIR /usr/gateway
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o build/gateway

FROM alpine:latest
COPY --from=build /usr/gateway/build/gateway /gateway
ENTRYPOINT ["/gateway"]
