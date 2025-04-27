PROTOC_GEN_GO := $(shell go env GOPATH)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(shell go env GOPATH)/bin/protoc-gen-go-grpc

generate:
	protoc --go_out=. --go-grpc_out=. proto/juego.proto

run-server:
	go run server/main.go

run-client:
	go run client/main.go
