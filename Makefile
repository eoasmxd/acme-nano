export PATH := $(GOPATH)/bin:$(PATH)

all: fmt build

build: acme-manual acme-cf acme-aliyun acme-dnspod acme-aws acme-gd acme-dgon acme-gcloud acme-namecheap acme-azure acme-ovh acme-hetznercloud acme-duckdns acme-porkbun acme-namesilo acme-vultr

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

acme-aws:
	go build -o bin/acme-aws ./cmd/aws

acme-gd:
	go build -o bin/acme-gd ./cmd/gd

acme-dgon:
	go build -o bin/acme-dgon ./cmd/dgon

acme-gcloud:
	go build -o bin/acme-gcloud ./cmd/gcloud

acme-namecheap:
	go build -o bin/acme-namecheap ./cmd/namecheap

acme-azure:
	go build -o bin/acme-azure ./cmd/azure

acme-ovh:
	go build -o bin/acme-ovh ./cmd/ovh

acme-hetznercloud:
	go build -o bin/acme-hetznercloud ./cmd/hetznercloud

acme-duckdns:
	go build -o bin/acme-duckdns ./cmd/duckdns

acme-porkbun:
	go build -o bin/acme-porkbun ./cmd/porkbun

acme-namesilo:
	go build -o bin/acme-namesilo ./cmd/namesilo

acme-vultr:
	go build -o bin/acme-vultr ./cmd/vultr

clean:
	rm -f ./bin/acme-manual
	rm -f ./bin/acme-cf
	rm -f ./bin/acme-aliyun
	rm -f ./bin/acme-dnspod
	rm -f ./bin/acme-aws
	rm -f ./bin/acme-gd
	rm -f ./bin/acme-dgon
	rm -f ./bin/acme-gcloud
	rm -f ./bin/acme-namecheap
	rm -f ./bin/acme-azure
	rm -f ./bin/acme-ovh
	rm -f ./bin/acme-hetznercloud
	rm -f ./bin/acme-duckdns
	rm -f ./bin/acme-porkbun
	rm -f ./bin/acme-namesilo
	rm -f ./bin/acme-vultr

