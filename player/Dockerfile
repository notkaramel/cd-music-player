FROM golang:1.24 as builder

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

ENV PATH="$PATH:$(go env GOPATH)/bin"

RUN protoc --proto_path=./protos --go_out=./player/protos/ --grpc_out=./player/protos/ protos/player.proto                                1 ↵ ✖ ✹ ✭