.PHONY: build tidy test

include Environment.mk
include Tools.mk

protogen: check-tools-protoc check-tools-protoc-gen-go

build: tidy
	go build -o dist/vandrare ./cmd/vandrare

tidy:
	go mod tidy
	go fmt ./...

test:
	go test ./...