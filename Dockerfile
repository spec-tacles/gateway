FROM golang:alpine AS build

WORKDIR /usr/gateway
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# https://github.com/valyala/gozstd/issues/20
RUN apk add gcc musl-dev libc-dev make && \
    GOZSTD_VER=$(cat go.mod | fgrep github.com/valyala/gozstd | awk '{print $NF}') && \
    go get -d github.com/valyala/gozstd@${GOZSTD_VER} && \
    cd ${GOPATH}/pkg/mod/github.com/valyala/gozstd@${GOZSTD_VER} && \
    chmod -R +w . && \
    make -j8 clean && \
    make -j8 libzstd.a && \
    cd /usr/gateway && \
    go build -o build/gateway

FROM alpine:latest
COPY --from=build /usr/gateway/build/gateway /gateway
ENTRYPOINT ["/gateway"]
