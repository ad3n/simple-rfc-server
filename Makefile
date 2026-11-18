SAPNWRFC_HOME ?= /usr/local/sap
export DYLD_LIBRARY_PATH := $(SAPNWRFC_HOME)/lib
export LD_LIBRARY_PATH := $(SAPNWRFC_HOME)/lib
export CGO_ENABLED := 1
export CGO_CFLAGS := -I$(SAPNWRFC_HOME)/include
export CGO_LDFLAGS := -L$(SAPNWRFC_HOME)/lib -Wl,-rpath,$(SAPNWRFC_HOME)/lib
export GOCACHE := /tmp/rfc-server-go-cache

.PHONY: run build test check
run:
	go run ./cmd/rfc-server -config mock.yml
build:
	go build -o bin/rfc-server ./cmd/rfc-server
	go build -o bin/rfc-call ./cmd/rfc-call
test:
	go test -race -count=1 ./...
check:
	betteralign -apply ./...
	betteralign ./...
	go vet ./...
