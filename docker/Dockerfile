# 构建阶段
FROM --platform=$BUILDPLATFORM golang:alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-manual ./cmd/manual && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-cf ./cmd/cloudflare && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-aliyun ./cmd/aliyun && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-dnspod ./cmd/dnspod && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-aws ./cmd/aws && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-gd ./cmd/gd && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-dgon ./cmd/dgon && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-gcloud ./cmd/gcloud && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-namecheap ./cmd/namecheap && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-azure ./cmd/azure && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-ovh ./cmd/ovh && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-hetznercloud ./cmd/hetznercloud && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-duckdns ./cmd/duckdns && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-porkbun ./cmd/porkbun && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-namesilo ./cmd/namesilo && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-vultr ./cmd/vultr

# 运行阶段
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/acme-manual /usr/local/bin/acme-manual
COPY --from=builder /bin/acme-cf /usr/local/bin/acme-cf
COPY --from=builder /bin/acme-aliyun /usr/local/bin/acme-aliyun
COPY --from=builder /bin/acme-dnspod /usr/local/bin/acme-dnspod
COPY --from=builder /bin/acme-aws /usr/local/bin/acme-aws
COPY --from=builder /bin/acme-gd /usr/local/bin/acme-gd
COPY --from=builder /bin/acme-dgon /usr/local/bin/acme-dgon
COPY --from=builder /bin/acme-gcloud /usr/local/bin/acme-gcloud
COPY --from=builder /bin/acme-namecheap /usr/local/bin/acme-namecheap
COPY --from=builder /bin/acme-azure /usr/local/bin/acme-azure
COPY --from=builder /bin/acme-ovh /usr/local/bin/acme-ovh
COPY --from=builder /bin/acme-hetznercloud /usr/local/bin/acme-hetznercloud
COPY --from=builder /bin/acme-duckdns /usr/local/bin/acme-duckdns
COPY --from=builder /bin/acme-porkbun /usr/local/bin/acme-porkbun
COPY --from=builder /bin/acme-namesilo /usr/local/bin/acme-namesilo
COPY --from=builder /bin/acme-vultr /usr/local/bin/acme-vultr

WORKDIR /certs
VOLUME ["/certs"]

