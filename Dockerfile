FROM golang

WORKDIR /usr/gateway
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o gate
CMD ["./gate"]
