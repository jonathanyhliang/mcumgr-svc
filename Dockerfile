FROM golang:alpine

RUN apk add socat

COPY ../ /root/mcumgr-svc

WORKDIR /root/mcumgr-svc

RUN go mod tidy \
    && go build .
