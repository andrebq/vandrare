.PHONY: build tidy test watch ci-make2png run

include Environment.mk
include Run.mk

build: tidy
	go build -o dist/vandrare ./cmd/vandrare

tidy:
	go mod tidy
	go fmt ./...

test:
	go test ./...

watch:
	modd -f modd.conf

ci-make2png:
	$(MAKE) -f ci-tools/make2png/Makefile run folder=$(PWD)
	$(MAKE) -f ci-tools/make2png/Makefile run folder=$(PWD)/examples/sample-data
	$(MAKE) -f ci-tools/make2png/Makefile run folder=$(PWD)/examples/new-key-registration
	$(MAKE) -f ci-tools/make2png/Makefile run folder=$(PWD)/examples/generate-client-config