.PHONY: tools-check-protoc tools-check-protoc-gen-go tools-check-curl tools-install-protoc-osx-arm tools-install-protoc-gen-go
.SILENT: tools-check-protoc tools-check-protoc-gen-go tools-check-curl

PROTOC_VERSION=26.1
curl_bin?=curl

tools-check-protoc: tools-check-curl
	which protoc 1>/dev/null || { echo "Missing protoc, call `make tools-install-protoc-<os>-<arch>" 1>&2; exit 1; }

tools-check-protoc-gen-go:
	which protoc-gen-go 1>/dev/null || { echo "Missing protoc-gen-go, call `make tools-install-protoc-gen-go" 1>&2; exit 1; }

tools-check-curl:
	which $(curl_bin) 1>/dev/null || { echo "Missing curl, please install it using your OS package manager and try again"; exit 1; }

tools-install-protoc-osx-arm:
	mkdir -p $(tools_bin)
	$(curl_bin) -fsSL -q "https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-osx-aarch_64.zip" > $(tools_bin) protoc.zip
	unzip -x 
