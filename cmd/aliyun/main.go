package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/eoasmxd/acme-nano/core"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// AliyunProvider implements core.DNSProvider using Aliyun Alidns OpenAPI (2015-01-09).
// AliyunProvider 封装阿里云云解析 DNS (Alidns 2015-01-09) 的 TXT 记录增删操作。
type AliyunProvider struct {
	accessKeyID     string
	accessKeySecret string
	domain          string
	lastRecordID    string
}

// percentEncode encodes strings following Aliyun OpenAPI RFC 3986 signing rules.
// percentEncode 按照阿里云 OpenAPI 签名规范对字符串进行百分号编码。
func percentEncode(s string) string {
	r := url.QueryEscape(s)
	r = strings.ReplaceAll(r, "+", "%20")
	r = strings.ReplaceAll(r, "*", "%2A")
	return strings.ReplaceAll(r, "%7E", "~")
}

// doRequest signs parameters with HMAC-SHA1 and sends an Alidns OpenAPI POST request.
// doRequest 使用 HMAC-SHA1 对请求参数签名并发送阿里云 DNS POST 请求。
func (a *AliyunProvider) doRequest(params map[string]string) ([]byte, error) {
	var nonce [8]byte
	_, _ = rand.Read(nonce[:])

	vals := url.Values{}
	vals.Set("Format", "JSON")
	vals.Set("Version", "2015-01-09")
	vals.Set("AccessKeyId", a.accessKeyID)
	vals.Set("SignatureMethod", "HMAC-SHA1")
	vals.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	vals.Set("SignatureVersion", "1.0")
	vals.Set("SignatureNonce", hex.EncodeToString(nonce[:]))
	for k, v := range params {
		vals.Set(k, v)
	}

	canonical := strings.NewReplacer("+", "%20", "*", "%2A", "%7E", "~").Replace(vals.Encode())
	stringToSign := "POST&%2F&" + percentEncode(canonical)

	mac := hmac.New(sha1.New, []byte(a.accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	vals.Set("Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	resp, err := httpClient.PostForm("https://alidns.aliyuncs.com/", vals)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Aliyun DNS API error [%d]: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// Present adds a TXT record to the configured Aliyun domain zone.
// Present 在指定阿里云域名下添加一条 TXT 记录。
func (a *AliyunProvider) Present(fqdn, value string) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(a.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	rr, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || rr == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}
	listBody, err := a.doRequest(map[string]string{
		"Action":     "DescribeSubDomainRecords",
		"DomainName": cleanZone,
		"SubDomain":  cleanFQDN,
		"Type":       "TXT",
	})
	if err != nil {
		return err
	}
	var listRes struct {
		DomainRecords struct {
			Record []struct {
				RecordID string `json:"RecordId"`
			} `json:"Record"`
		} `json:"DomainRecords"`
	}
	if err := json.Unmarshal(listBody, &listRes); err != nil {
		return fmt.Errorf("failed to parse Aliyun DNS records: %s", string(listBody))
	}
	for _, rec := range listRes.DomainRecords.Record {
		if _, err := a.doRequest(map[string]string{
			"Action":   "DeleteDomainRecord",
			"RecordId": rec.RecordID,
		}); err != nil {
			return err
		}
	}
	body, err := a.doRequest(map[string]string{
		"Action":     "AddDomainRecord",
		"DomainName": cleanZone,
		"RR":         rr,
		"Type":       "TXT",
		"Value":      value,
	})
	if err != nil {
		return err
	}
	var res struct {
		RecordID string `json:"RecordId"`
	}
	if err := json.Unmarshal(body, &res); err != nil || res.RecordID == "" {
		return fmt.Errorf("failed to parse Aliyun RecordId: %s", string(body))
	}
	a.lastRecordID = res.RecordID
	return nil
}

// CleanUp deletes the previously created TXT record by its stored RecordId.
// CleanUp 根据内存中记录的 RecordId 删除此前创建的 TXT 记录。
func (a *AliyunProvider) CleanUp(fqdn, value string) error {
	if a.lastRecordID == "" {
		return nil
	}
	id := a.lastRecordID
	a.lastRecordID = ""
	_, err := a.doRequest(map[string]string{
		"Action":   "DeleteDomainRecord",
		"RecordId": id,
	})
	return err
}

func main() {
	provider := &AliyunProvider{
		accessKeyID:     os.Getenv("DNS_ID"),
		accessKeySecret: os.Getenv("DNS_SECRET"),
		domain:          os.Getenv("DNS_ZONE"),
	}
	if provider.accessKeyID == "" || provider.accessKeySecret == "" || provider.domain == "" {
		core.LogError("Error: environment variables DNS_ID, DNS_SECRET, and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}
