# acme-nano

[English](./README.md) | 中文文档

基于 **Go 纯标准库（零第三方依赖）**、**DNS-01 专属**、**按需单文件入口** 以及 **极致可审查性** 构建的极简 ACME 客户端。

---

## 一、 工程目录与架构边界

```text
github.com/eoasmxd/acme-nano/
├── go.mod                      # 仅声明 module 与 Go 版本，零外部依赖
├── README.md                   # 英文主文档
├── README_zh.md                # 中文说明文档
├── AUDIT.md                    # 英文安全审计指南
├── AUDIT_zh.md                 # 中文安全审计指南
├── core/                       # [ACME 内核] 纯 RFC 8555 协议实现
│   ├── acme.go                 # 核心状态机：证书到期检测、账户注册、下单、串行挑战、CSR 签发
│   ├── ca.go                   # 内置主流 ACME v2 发证机构 (CA) 预设目录与解析器
│   ├── jws.go                  # 密码学封装：ECDSA P-256 签名、JWK Thumbprint、Replay-Nonce 管理
│   └── dns_check.go            # 纯标准库 net.LookupTXT 轮询，探测 DNS 传播状态
└── cmd/                        # [主入口层] 每个目录是一个独立可审计的单文件程序
    ├── manual/main.go          # 零 API 依赖：终端打印 TXT 记录并等待回车（调试与兜底）
    ├── cloudflare/main.go      # Cloudflare API v4 Token 驱动入口
    ├── aliyun/main.go          # 阿里云云解析 Alidns (HMAC-SHA1) 驱动入口
    └── dnspod/main.go          # 腾讯云 DNSPod API v3 (TC3-HMAC-SHA256) 驱动入口
```

### 核心设计特点

1. **零供应链风险**：`go.mod` 无任何 `require`，完全基于 Go 标准库实现。
2. **内核与驱动严格隔离**：`core/` 不包含任何云厂商域名或 API 逻辑；每个 `cmd/<provider>/main.go` 的凭证字段仅在本地私有可见，按需编译出专属单二进制文件。
3. **串行验证防冲突**：同时申请主域名与通配符（如 `example.com,*.example.com`）时，内核依次完成每个域名的验证与清理，避免同名 TXT 记录互相覆盖，同时仅需最小化的增删记录 API 权限。
4. **内置幂等续期检查**：若本地目标证书（默认 `fullchain.pem`）已存在、包含所请求的全部域名且剩余有效期大于 30 天，直接跳过签发，可安全放入每日 Cron 定时任务。

---

## 二、 快速编译与使用

您可以根据自身安全审计与部署习惯，选择以下三种方式之一获取并运行 `acme-nano`（所有入口二进制命名规则可对照 [Makefile](./Makefile)）：

### 方式一：从源码按需编译（推荐用于自主审计）

仅编译自己所需 DNS 服务商的单文件入口：

```bash
go build -o acme-cf ./cmd/cloudflare
# 或一次性编译全部入口到 bin/ 目录
make build
```

### 方式二：下载官方预编译二进制

直接前往 [GitHub Releases](https://github.com/eoasmxd/acme-nano/releases) 下载对应平台与架构的官方压缩包（`acme-nano_<version>_<os>_<arch>.tar.gz` 或 `.zip`），解压后即可按需使用其中的 `acme-<provider>` 单文件二进制：

```bash
tar -xzf acme-nano_<version>_linux_amd64.tar.gz
cd acme-nano_<version>_linux_amd64
./acme-cf -d "example.com,*.example.com"
```

### 方式三：使用官方 Docker 镜像

官方多架构镜像内置了全部 `acme-<provider>` 入口，默认工作目录与挂载卷为 `/certs`（证书及 `account.key` 将直接保存在宿主机挂载目录中）：

- **GitHub Container Registry (GHCR)**：`ghcr.io/eoasmxd/acme-nano:latest`
- **Docker Hub**：`eoasmxd/acme-nano:latest`

```bash
docker run --rm -v "$(pwd)/certs:/certs" \
  -e DNS_SECRET="你的_Cloudflare_API_Token" \
  -e DNS_ZONE="你的_Zone_ID" \
  ghcr.io/eoasmxd/acme-nano:latest \
  acme-cf -d "example.com,*.example.com"
```

---

### 1. 手动交互模式 (`cmd/manual`)

适用于首次联调协议或长尾 DNS 服务商：

```bash
./acme-manual -ca letsencrypt-staging -d "example.com,*.example.com"
```

### 2. Cloudflare 模式 (`cmd/cloudflare`)

```bash
export DNS_SECRET="你的_Cloudflare_API_Token"
export DNS_ZONE="你的_Zone_ID"

./acme-cf -d "example.com,*.example.com"
```

### 3. 阿里云 DNS 模式 (`cmd/aliyun`)

```bash
export DNS_ID="你的_AccessKeyId"
export DNS_SECRET="你的_AccessKeySecret"
export DNS_ZONE="example.com"

./acme-aliyun -d "example.com,*.example.com"
```

### 4. 腾讯云 DNSPod 模式 (`cmd/dnspod`)

```bash
export DNS_ID="你的_SecretId"
export DNS_SECRET="你的_SecretKey"
export DNS_ZONE="example.com"

./acme-dnspod -d "example.com,*.example.com"
```

---

## 三、 统一环境变量对照

所有需要 API 凭证的入口均使用统一的环境变量 `DNS_ID`、`DNS_SECRET`、`DNS_ZONE`：

| DNS 服务商 | `DNS_ID` | `DNS_SECRET` | `DNS_ZONE` |
| :--- | :--- | :--- | :--- |
| Cloudflare (`cmd/cloudflare`) | 无需设置 | Cloudflare API Token | Zone ID |
| 阿里云 DNS (`cmd/aliyun`) | AccessKeyId | AccessKeySecret | 主域名（如 `example.com`） |
| 腾讯云 DNSPod (`cmd/dnspod`) | SecretId | SecretKey | 主域名（如 `example.com`） |
| AWS Route 53 (`cmd/aws`) | AWS Access Key ID | AWS Secret Access Key | Hosted Zone ID（如 `Z123456`） |
| GoDaddy (`cmd/gd`) | API Key | API Secret | 主域名（如 `example.com`） |
| DigitalOcean (`cmd/dgon`) | 无需设置 | DigitalOcean API Token | 主域名（如 `example.com`） |
| Google Cloud DNS (`cmd/gcloud`) | GCP Project ID | OAuth2 Bearer Token 或 Service Account JSON | Managed Zone 名称 |
| Namecheap (`cmd/namecheap`) | `ApiUser:ClientIp`（省略 IP 时默认 `127.0.0.1`） | Namecheap API Key | 主域名（如 `example.com`） |
| Microsoft Azure DNS (`cmd/azure`) | `TenantID/ClientID` | Client Secret | `SubscriptionID/ResourceGroup/ZoneName` |
| OVHcloud (`cmd/ovh`) | Application Key (`AK`) | `ApplicationSecret:ConsumerKey` (`AS:CK`) | 主域名（或 `ovh-us/example.com` 指定端点） |
| Hetzner Cloud DNS (`cmd/hetznercloud`) | 无需设置 | Hetzner Cloud API Token | 主域名（如 `example.com`） |
| DuckDNS (`cmd/duckdns`) | 无需设置 | DuckDNS Token | 可选，主域名（不设则从 `-d` 自动提取） |
| Porkbun (`cmd/porkbun`) | API Key | Secret API Key | 主域名（如 `example.com`） |
| NameSilo (`cmd/namesilo`) | 无需设置 | NameSilo API Key | 主域名（如 `example.com`） |
| Vultr (`cmd/vultr`) | 无需设置 | Vultr API Key | 主域名（如 `example.com`） |

当使用 ZeroSSL 或 Google CA 等需要外部账户绑定（EAB）的发证机构时，可额外设置以下环境变量：

| 环境变量 | 说明 |
| :--- | :--- |
| `EAB_KID` | External Account Binding Key ID |
| `EAB_HMAC` | External Account Binding HMAC 密钥 |

---

## 四、 通用命令行参数与 CA 预设

所有入口均支持以下统一参数：

| 参数 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `-d` | 必填 | 逗号分隔的域名列表，如 `example.com,*.example.com` |
| `-ca` | `letsencrypt` | 发证机构预设名称或自定义 HTTPS ACME Directory URL |
| `-account` | `account.key` | ACME 账户 ECDSA P-256 私钥文件路径（不存在则自动创建） |
| `-cert` | `fullchain.pem` | 签发后的完整证书链 PEM 输出路径 |
| `-key` | `privkey.pem` | 证书 ECDSA P-256 私钥文件路径（不存在则自动创建） |

支持的 `-ca` 预设值（集中定义在 [core/ca.go](./core/ca.go)）：

- `letsencrypt`
- `letsencrypt-staging`
- `buypass` / `buypass-staging`
- `zerossl`
- `google` / `google-staging`
- 或任意以 `https://` 开头的自定义 ACME v2 Directory URL

