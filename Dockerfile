FROM golang:1.19

COPY . /root/mcumgr-svc

WORKDIR /root/mcumgr-svc

RUN go mod tidy \
    && go build .

EXPOSE 8081

CMD ["./mcumgr-svc", "-p /dev/ttyACM0"]
