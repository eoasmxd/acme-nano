# acme-nano 安全审计指南

[English](./AUDIT.md) | 中文审计指南

`acme-nano` 的设计目标是让任何具备基础 Go 阅读能力的开发者在 **5 分钟内完成全量代码安全审计**，彻底杜绝 DNS API 密钥泄露与供应链投毒风险。

---

## 一、 外部依赖审计（耗时：10 秒）

检查 [go.mod](./go.mod)：

- 文件中仅包含 `module github.com/eoasmxd/acme-nano` 与 Go 版本声明。
- **无任何 `require` 块**，不存在 `go.sum`，编译过程不会下载任何第三方包。

---

## 二、 ACME 内核隔离性审计（耗时：2 分钟）

内核位于 `core/` 目录，仅包含四个文件：

1. [core/ca.go](./core/ca.go)：
   - 仅维护主流 ACME v2 发证机构（`letsencrypt`、`buypass`、`zerossl`、`google` 及其测试环境）官方 Directory URL 预设表与解析逻辑。
2. [core/jws.go](./core/jws.go)：
   - 仅负责读取/生成本地 ECDSA P-256 PEM 私钥（权限 `0600`）、计算 RFC 7638 JWK 指纹与 ES256 (`R || S` 64 字节拼接) 签名。
   - 网络请求仅发往由 `core/ca.go` 解析出的 ACME Directory 及各协议端点。
3. [core/dns_check.go](./core/dns_check.go)：
   - 仅调用 Go 标准库 `net.LookupTXT` 轮询验证记录传播状态。
4. [core/acme.go](./core/acme.go)：
   - 仅负责编排 ACME 协议签发与续期状态机，不包含任何 DNS 云厂商逻辑，也无法接触各厂商凭证。
   - 各 `cmd/<provider>/main.go` 中的 DNS 凭证字段均为包级私有，在 Go 语法层面与 `core/` 严格隔离。
   - 通过 `DNSProvider` 接口仅向入口层传递脱敏后的 `fqdn`（如 `_acme-challenge.example.com`）和挑战哈希值 `value`。
   - 若超时未探测到本地 DNS 记录，仅输出警告并继续交由 CA 权威服务器验证，不会中断清理流程。

---

## 三、 按需审计单文件 DNS 入口（耗时：1 分钟/入口）

您只需审查自己实际编译的那个 `cmd/<provider>/main.go` 文件：

- **Cloudflare** ([cmd/cloudflare/main.go](./cmd/cloudflare/main.go))：
  - 读取环境变量：`DNS_SECRET`（API Token）、`DNS_ZONE`（Zone ID）。
  - 唯一网络通信目标：`https://api.cloudflare.com/client/v4/zones/{ZoneID}/dns_records`。
- **阿里云 DNS** ([cmd/aliyun/main.go](./cmd/aliyun/main.go))：
  - 读取环境变量：`DNS_ID`（AccessKeyId）、`DNS_SECRET`（AccessKeySecret）、`DNS_ZONE`（主域名）。
  - 唯一网络通信目标：`https://alidns.aliyuncs.com/`（仅调用 `AddDomainRecord` 与 `DeleteDomainRecord`）。
- **腾讯云 DNSPod** ([cmd/dnspod/main.go](./cmd/dnspod/main.go))：
  - 读取环境变量：`DNS_ID`（SecretId）、`DNS_SECRET`（SecretKey）、`DNS_ZONE`（主域名）。
  - 唯一网络通信目标：`https://dnspod.tencentcloudapi.com/`（仅调用 `CreateRecord` 与 `DeleteRecord`）。
- **手动模式** ([cmd/manual/main.go](./cmd/manual/main.go))：
  - 零环境变量读取、零外部 API 请求，完全通过标准输入/输出交互。
- **其他内置 DNS 入口**（如 `cmd/aws`、`cmd/gd`、`cmd/dgon`、`cmd/gcloud`、`cmd/namecheap`、`cmd/azure`、`cmd/ovh`、`cmd/hetznercloud`、`cmd/duckdns`、`cmd/porkbun`、`cmd/namesilo`、`cmd/vultr` 等，完整清单见 [README.zh.md](./README.zh.md)）：
  - 均遵循相同的单文件最小化隔离结构：仅按需读取统一环境变量 `DNS_ID`、`DNS_SECRET`、`DNS_ZONE`，且仅向对应 DNS 服务商的官方 API 端点发起 TXT 记录增删（或认证换取 Token）请求，无任何额外遥测或跨厂商通信。

