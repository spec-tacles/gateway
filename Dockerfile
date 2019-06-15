FROM golang AS build

WORKDIR /usr/gateway
RUN apt-get update && \
	apt-get install -y xz-utils
ADD https://github.com/upx/upx/releases/download/v3.95/upx-3.95-amd64_linux.tar.xz /usr/local
RUN xz -d -c /usr/local/upx-3.95-amd64_linux.tar.xz | \
	tar -xOf - upx-3.95-amd64_linux/upx > /bin/upx && \
	chmod a+x /bin/upx
ENV GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o build/gateway && \
	upx build/gateway

FROM debian AS runtime
RUN apt-get update -y && apt-get install -y ca-certificates
COPY --from=build /usr/gateway/build/gateway /gateway
ENTRYPOINT ["/gateway"]
