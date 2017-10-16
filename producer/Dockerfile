FROM golang:latest

RUN mkdir -p /go/src/producer
ADD . /go/src/producer

WORKDIR /go/src/producer
RUN export GOPATH=/go
RUN go get
RUN go build -o producer .

CMD ["/go/src/producer/producer"]