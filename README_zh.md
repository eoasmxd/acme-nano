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

### 1. 手动交互模式 (`cmd/manual`)

适用于首次联调协议或长尾 DNS 服务商：

```bash
go build -o acme-manual ./cmd/manual
./acme-manual -ca letsencrypt-staging -d "example.com,*.example.com"
```

### 2. Cloudflare 模式 (`cmd/cloudflare`)

```bash
go build -o acme-cf ./cmd/cloudflare

export DNS_SECRET="你的_Cloudflare_API_Token"
export DNS_ZONE="你的_Zone_ID"

./acme-cf -d "example.com,*.example.com"
```

### 3. 阿里云 DNS 模式 (`cmd/aliyun`)

```bash
go build -o acme-aliyun ./cmd/aliyun

export DNS_ID="你的_AccessKeyId"
export DNS_SECRET="你的_AccessKeySecret"
export DNS_ZONE="example.com"

./acme-aliyun -d "example.com,*.example.com"
```

### 4. 腾讯云 DNSPod 模式 (`cmd/dnspod`)

```bash
go build -o acme-dnspod ./cmd/dnspod

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
| Cloudflare | 无需设置 | Cloudflare API Token | Zone ID |
| 阿里云 DNS | AccessKeyId | AccessKeySecret | 主域名（如 `example.com`） |
| 腾讯云 DNSPod | SecretId | SecretKey | 主域名（如 `example.com`） |

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

