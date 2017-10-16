FROM golang:latest

RUN mkdir -p /go/src/consumer
ADD . /go/src/consumer

WORKDIR /go/src/consumer
RUN export GOPATH=/go
RUN go get
RUN go build -o consumer .

CMD ["/go/src/consumer/consumer"]