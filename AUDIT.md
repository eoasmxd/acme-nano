# acme-nano Security Audit Guide

English | [中文审计指南](./AUDIT.zh.md)

`acme-nano` is designed so that any engineer familiar with basic Go can **audit the entire codebase within 5 minutes**, eliminating supply-chain risks and DNS credential leakage.

---

## 1. Dependency Audit (~10 seconds)

Inspect [go.mod](./go.mod):

- Contains only `module github.com/eoasmxd/acme-nano` and the Go toolchain version.
- **Zero `require` directives** and no `go.sum` file; building downloads zero external packages.

---

## 2. ACME Core Isolation Audit (~2 minutes)

The core resides in `core/` and consists of four focused files:

1. [core/ca.go](./core/ca.go):
   - Pure lookup table mapping preset CA names (`letsencrypt`, `buypass`, `zerossl`, `google`, and staging endpoints) to official ACME v2 Directory URLs.
2. [core/jws.go](./core/jws.go):
   - Reads/creates local ECDSA P-256 PEM keys (`0600` permissions), computes RFC 7638 JWK thumbprints, and produces ES256 (`R || S` 64-byte) signatures.
   - HTTP requests go strictly to the ACME Directory endpoints resolved by `core/ca.go`.
3. [core/dns_check.go](./core/dns_check.go):
   - Uses only `net.LookupTXT` from the Go standard library to poll DNS propagation.
4. [core/acme.go](./core/acme.go):
   - Orchestrates only the ACME issuance and renewal state machine, without any DNS provider logic or access to provider credentials.
   - All DNS provider credential fields in `cmd/<provider>/main.go` are unexported and isolated at the Go language level.
   - Passes only the challenge `fqdn` (e.g. `_acme-challenge.example.com`) and hashed TXT `value` to the `DNSProvider` interface.
   - On propagation check timeout, prints a warning and proceeds with CA validation without skipping `CleanUp`.

---

## 3. Single-File Provider Entrypoint Audit (~1 minute per provider)

You only need to review the `cmd/<provider>/main.go` file you compile:

- **Cloudflare** ([cmd/cloudflare/main.go](./cmd/cloudflare/main.go)):
  - Environment variables read: `DNS_SECRET` (API Token), `DNS_ZONE` (Zone ID).
  - Sole network destination: `https://api.cloudflare.com/client/v4/zones/{ZoneID}/dns_records`.
- **Aliyun DNS** ([cmd/aliyun/main.go](./cmd/aliyun/main.go)):
  - Environment variables read: `DNS_ID` (AccessKeyId), `DNS_SECRET` (AccessKeySecret), `DNS_ZONE` (apex domain).
  - Sole network destination: `https://alidns.aliyuncs.com/` (`AddDomainRecord` and `DeleteDomainRecord` only).
- **Tencent Cloud DNSPod** ([cmd/dnspod/main.go](./cmd/dnspod/main.go)):
  - Environment variables read: `DNS_ID` (SecretId), `DNS_SECRET` (SecretKey), `DNS_ZONE` (apex domain).
  - Sole network destination: `https://dnspod.tencentcloudapi.com/` (`CreateRecord` and `DeleteRecord` only).
- **Manual Mode** ([cmd/manual/main.go](./cmd/manual/main.go)):
  - Reads zero environment variables and makes zero provider API calls; communicates solely via stdin/stdout.
- **Other Built-in DNS Entrypoints** (e.g. `cmd/aws`, `cmd/gd`, `cmd/dgon`, `cmd/gcloud`, `cmd/namecheap`, `cmd/azure`, `cmd/ovh`, `cmd/hetznercloud`, `cmd/duckdns`, `cmd/porkbun`, `cmd/namesilo`, `cmd/vultr`; see [README.md](./README.md) for the full list):
  - All follow the same minimal single-file isolation pattern: they read only the unified `DNS_ID`, `DNS_SECRET`, and `DNS_ZONE` environment variables as needed and communicate strictly with their respective provider's official API endpoints for TXT record management (and token exchange where required), with zero telemetry or cross-provider traffic.
