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
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build -ldflags "-s -w" -o /bin/acme-dnspod ./cmd/dnspod

# 运行阶段
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/acme-manual /usr/local/bin/acme-manual
COPY --from=builder /bin/acme-cf /usr/local/bin/acme-cf
COPY --from=builder /bin/acme-aliyun /usr/local/bin/acme-aliyun
COPY --from=builder /bin/acme-dnspod /usr/local/bin/acme-dnspod

WORKDIR /certs
VOLUME ["/certs"]

