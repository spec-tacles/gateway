FROM golang AS build

WORKDIR /usr/gateway
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o build/gateway

FROM debian:buster-slim AS runtime
RUN apt-get update -y && apt-get install -y ca-certificates
COPY --from=build /usr/gateway/build/gateway /gateway
ENTRYPOINT ["/gateway"]
