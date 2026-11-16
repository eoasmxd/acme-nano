# acme-nano

English | [中文文档](./README.zh.md)

A minimal ACME v2 client built exclusively with the **Go standard library (zero third-party dependencies)**, focused on **DNS-01 challenges**, **on-demand single-file entrypoints**, and **rapid code auditability**.

---

## 1. Architecture & Boundaries

```text
github.com/eoasmxd/acme-nano/
├── go.mod                      # Module declaration only; zero external dependencies
├── README.md                   # English documentation (primary)
├── README.zh.md                # Chinese documentation
├── AUDIT.md                    # English security audit guide (primary)
├── AUDIT.zh.md                 # Chinese security audit guide
├── core/                       # [ACME Core] Pure RFC 8555 protocol implementation
│   ├── acme.go                 # State machine: expiry check, account, order, serial challenge, CSR
│   ├── ca.go                   # Built-in ACME v2 CA directory presets & resolver
│   ├── jws.go                  # Cryptography: ECDSA P-256 signing, JWK Thumbprint, Replay-Nonce
│   └── dns_check.go            # Standard library net.LookupTXT propagation poller
└── cmd/                        # [Entrypoints] Standalone auditable single-file binaries
    ├── manual/main.go          # Zero API dependencies: interactive terminal TXT prompt
    ├── cloudflare/main.go      # Cloudflare API v4 Token provider
    ├── aliyun/main.go          # Aliyun Alidns (HMAC-SHA1) provider
    └── dnspod/main.go          # Tencent Cloud DNSPod API v3 (TC3-HMAC-SHA256) provider
```

### Design Highlights

1. **Zero Supply-Chain Risk**: `go.mod` contains zero `require` statements and relies solely on the Go standard library.
2. **Strict Core Isolation**: `core/` contains no DNS provider domains or API logic. Each `cmd/<provider>/main.go` holds its provider credentials in unexported fields and compiles into an independent binary.
3. **Serial Challenge Execution**: When requesting both an apex domain and a wildcard (`example.com,*.example.com`), domains are validated and cleaned up one at a time, preventing TXT record overwrite conflicts while requiring only minimal create/delete API permissions.
4. **Idempotent Renewal Check**: If the target certificate (`fullchain.pem` by default) exists, covers all requested domains, and has more than 30 days of validity remaining, issuance is skipped automatically.

---

## 2. Quick Start

Choose one of the following three ways to obtain and run `acme-nano` depending on your audit and deployment workflow (see [Makefile](./Makefile) for the binary names of all entrypoints):

### Option 1: Build from Source on Demand (Recommended for Self-Audit)

Compile only the single-file entrypoint for your DNS provider:

```bash
go build -o acme-cf ./cmd/cloudflare
# Or build all entrypoints into bin/
make build
```

### Option 2: Download Official Prebuilt Binaries

Download the release archive matching your OS and architecture (`acme-nano_<version>_<os>_<arch>.tar.gz` or `.zip`) from [GitHub Releases](https://github.com/eoasmxd/acme-nano/releases), extract it, and run the desired `acme-<provider>` binary directly:

```bash
tar -xzf acme-nano_<version>_linux_amd64.tar.gz
cd acme-nano_<version>_linux_amd64
./acme-cf -d "example.com,*.example.com"
```

### Option 3: Use the Official Docker Image

The official multi-arch image includes all `acme-<provider>` binaries with `/certs` as the default `WORKDIR` and `VOLUME` (so generated certificates and `account.key` persist directly in your mounted host directory):

- **GitHub Container Registry (GHCR)**: `ghcr.io/eoasmxd/acme-nano:latest`
- **Docker Hub**: `eoasmxd/acme-nano:latest`

```bash
docker run --rm -v "$(pwd)/certs:/certs" \
  -e DNS_SECRET="your_cloudflare_api_token" \
  -e DNS_ZONE="your_zone_id" \
  ghcr.io/eoasmxd/acme-nano:latest \
  acme-cf -d "example.com,*.example.com"
```

---

### 1. Manual Mode (`cmd/manual`)

Useful for protocol testing or DNS providers without API automation:

```bash
./acme-manual -ca letsencrypt-staging -d "example.com,*.example.com"
```

### 2. Cloudflare Mode (`cmd/cloudflare`)

```bash
export DNS_SECRET="your_cloudflare_api_token"
export DNS_ZONE="your_zone_id"

./acme-cf -d "example.com,*.example.com"
```

### 3. Aliyun DNS Mode (`cmd/aliyun`)

```bash
export DNS_ID="your_access_key_id"
export DNS_SECRET="your_access_key_secret"
export DNS_ZONE="example.com"

./acme-aliyun -d "example.com,*.example.com"
```

### 4. Tencent Cloud DNSPod Mode (`cmd/dnspod`)

```bash
export DNS_ID="your_secret_id"
export DNS_SECRET="your_secret_key"
export DNS_ZONE="example.com"

./acme-dnspod -d "example.com,*.example.com"
```

---

## 3. Unified Environment Variables

All API-based entrypoints use the unified environment variables `DNS_ID`, `DNS_SECRET`, and `DNS_ZONE`:

| DNS Provider | `DNS_ID` | `DNS_SECRET` | `DNS_ZONE` |
| :--- | :--- | :--- | :--- |
| Cloudflare (`cmd/cloudflare`) | Not required | Cloudflare API Token | Zone ID |
| Aliyun DNS (`cmd/aliyun`) | AccessKeyId | AccessKeySecret | Apex domain (e.g. `example.com`) |
| Tencent Cloud DNSPod (`cmd/dnspod`) | SecretId | SecretKey | Apex domain (e.g. `example.com`) |
| AWS Route 53 (`cmd/aws`) | AWS Access Key ID | AWS Secret Access Key | Hosted Zone ID (e.g. `Z123456`) |
| GoDaddy (`cmd/gd`) | API Key | API Secret | Apex domain (e.g. `example.com`) |
| DigitalOcean (`cmd/dgon`) | Not required | DigitalOcean API Token | Apex domain (e.g. `example.com`) |
| Google Cloud DNS (`cmd/gcloud`) | GCP Project ID | OAuth2 Bearer Token or Service Account JSON | Managed Zone name |
| Namecheap (`cmd/namecheap`) | `ApiUser:ClientIp` (defaults to `127.0.0.1` if omitted) | Namecheap API Key | Apex domain (e.g. `example.com`) |
| Microsoft Azure DNS (`cmd/azure`) | `TenantID/ClientID` | Client Secret | `SubscriptionID/ResourceGroup/ZoneName` |
| OVHcloud (`cmd/ovh`) | Application Key (`AK`) | `ApplicationSecret:ConsumerKey` (`AS:CK`) | Apex domain (or `ovh-us/example.com` with endpoint) |
| Hetzner Cloud DNS (`cmd/hetznercloud`) | Not required | Hetzner Cloud API Token | Apex domain (e.g. `example.com`) |
| DuckDNS (`cmd/duckdns`) | Not required | DuckDNS Token | Optional, domain (auto-extracted from `-d` if unset) |
| Porkbun (`cmd/porkbun`) | API Key | Secret API Key | Apex domain (e.g. `example.com`) |
| NameSilo (`cmd/namesilo`) | Not required | NameSilo API Key | Apex domain (e.g. `example.com`) |
| Vultr (`cmd/vultr`) | Not required | Vultr API Key | Apex domain (e.g. `example.com`) |

Optional CA EAB (External Account Binding) environment variables for ZeroSSL / Google CA:

| Variable | Description |
| :--- | :--- |
| `EAB_KID` | External Account Binding Key ID |
| `EAB_HMAC` | External Account Binding HMAC key |

---

## 4. Common CLI Flags & CA Presets

All entrypoints support the following CLI flags:

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-d` | Required | Comma-separated list of domains (e.g. `example.com,*.example.com`) |
| `-ca` | `letsencrypt` | CA preset name or custom HTTPS ACME directory URL |
| `-account` | `account.key` | Path to ACME account ECDSA P-256 key (auto-generated if absent) |
| `-cert` | `fullchain.pem` | Output path for the issued certificate chain PEM |
| `-key` | `privkey.pem` | Path to certificate ECDSA P-256 private key (auto-generated if absent) |

Supported `-ca` preset values (defined in [core/ca.go](./core/ca.go)):

- `letsencrypt`
- `letsencrypt-staging`
- `buypass` / `buypass-staging`
- `zerossl`
- `google` / `google-staging`
- Or any custom ACME v2 directory URL starting with `https://`
