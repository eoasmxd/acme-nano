export PATH := $(GOPATH)/bin:$(PATH)

all: fmt build

build: acme-manual acme-cf acme-aliyun acme-dnspod

fmt:
	go fmt ./...

acme-manual:
	go build -o bin/acme-manual ./cmd/manual

acme-cf:
	go build -o bin/acme-cf ./cmd/cloudflare

acme-aliyun:
	go build -o bin/acme-aliyun ./cmd/aliyun

acme-dnspod:
	go build -o bin/acme-dnspod ./cmd/dnspod

clean:
	rm -f ./bin/acme-manual
	rm -f ./bin/acme-cf
	rm -f ./bin/acme-aliyun
	rm -f ./bin/acme-dnspod

