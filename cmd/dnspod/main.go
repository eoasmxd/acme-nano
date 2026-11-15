package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eoasmxd/acme-nano/core"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// DNSPodProvider implements core.DNSProvider using Tencent Cloud DNSPod API v3 (2021-03-23).
// DNSPodProvider 封装腾讯云 DNSPod API v3 (2021-03-23) 的 TXT 记录增删操作。
type DNSPodProvider struct {
	secretID     string
	secretKey    string
	domain       string
	lastRecordID uint64
}

func hmacSHA256(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// doRequest signs and dispatches a Tencent Cloud API v3 request using TC3-HMAC-SHA256.
// doRequest 使用 TC3-HMAC-SHA256 对请求进行签名并调用腾讯云 DNSPod API v3 接口。
func (d *DNSPodProvider) doRequest(action string, params map[string]any) ([]byte, error) {
	payload, _ := json.Marshal(params)
	ts := time.Now().Unix()
	date := time.Unix(ts, 0).UTC().Format("2006-01-02")

	canonicalRequest := "POST\n/\n\ncontent-type:application/json\nhost:dnspod.tencentcloudapi.com\nx-tc-action:" +
		strings.ToLower(action) + "\n\ncontent-type;host;x-tc-action\n" + sha256Hex(payload)
	scope := date + "/dnspod/tc3_request"
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s\n%s", ts, scope, sha256Hex([]byte(canonicalRequest)))

	secretDate := hmacSHA256([]byte("TC3"+d.secretKey), date)
	secretService := hmacSHA256(secretDate, "dnspod")
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	req, err := http.NewRequest(http.MethodPost, "https://dnspod.tencentcloudapi.com/", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "dnspod.tencentcloudapi.com")
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", "2021-03-23")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("Authorization", fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host;x-tc-action, Signature=%s",
		d.secretID, scope, signature,
	))

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var errorResp struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if resp.StatusCode >= 300 || (json.Unmarshal(body, &errorResp) == nil && errorResp.Response.Error != nil) {
		return nil, fmt.Errorf("Tencent Cloud DNSPod API error [%d]: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// Present creates a TXT record in the configured Tencent Cloud DNSPod domain.
// Present 在指定腾讯云 DNSPod 域名下创建一条 TXT 记录。
func (d *DNSPodProvider) Present(fqdn, value string) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(d.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	subDomain, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || subDomain == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}
	listBody, err := d.doRequest("DescribeRecordList", map[string]any{
		"Domain":     cleanZone,
		"Subdomain":  subDomain,
		"RecordType": "TXT",
	})
	if err != nil {
		if !strings.Contains(err.Error(), "ResourceNotFound.NoDataOfRecord") {
			return err
		}
	} else {
		var listRes struct {
			Response struct {
				RecordList []struct {
					RecordID uint64 `json:"RecordId"`
				} `json:"RecordList"`
			} `json:"Response"`
		}
		if err := json.Unmarshal(listBody, &listRes); err != nil {
			return fmt.Errorf("failed to parse Tencent Cloud DNSPod RecordList: %s", string(listBody))
		}
		for _, rec := range listRes.Response.RecordList {
			if _, err := d.doRequest("DeleteRecord", map[string]any{
				"Domain":   cleanZone,
				"RecordId": rec.RecordID,
			}); err != nil {
				return err
			}
		}
	}
	body, err := d.doRequest("CreateRecord", map[string]any{
		"Domain":     cleanZone,
		"SubDomain":  subDomain,
		"RecordType": "TXT",
		"RecordLine": "默认",
		"Value":      value,
	})
	if err != nil {
		return err
	}
	var res struct {
		Response struct {
			RecordID uint64 `json:"RecordId"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &res); err != nil || res.Response.RecordID == 0 {
		return fmt.Errorf("failed to parse Tencent Cloud DNSPod RecordId: %s", string(body))
	}
	d.lastRecordID = res.Response.RecordID
	return nil
}

// CleanUp deletes the previously created TXT record by its stored RecordId.
// CleanUp 根据内存中记录的 RecordId 删除此前创建的 TXT 记录。
func (d *DNSPodProvider) CleanUp(fqdn, value string) error {
	if d.lastRecordID == 0 {
		return nil
	}
	id := d.lastRecordID
	d.lastRecordID = 0
	cleanZone := strings.ToLower(strings.TrimSuffix(d.domain, "."))
	_, err := d.doRequest("DeleteRecord", map[string]any{
		"Domain":   cleanZone,
		"RecordId": id,
	})
	return err
}

func main() {
	provider := &DNSPodProvider{
		secretID:  os.Getenv("DNS_ID"),
		secretKey: os.Getenv("DNS_SECRET"),
		domain:    os.Getenv("DNS_ZONE"),
	}
	if provider.secretID == "" || provider.secretKey == "" || provider.domain == "" {
		fmt.Fprintln(os.Stderr, "Error: environment variables DNS_ID, DNS_SECRET, and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}
