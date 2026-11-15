package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eoasmxd/acme-nano/core"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// AWSProvider 封装 Amazon Route 53 API (2013-04-01) 的 TXT 记录增删操作。
type AWSProvider struct {
	accessKeyID     string
	secretAccessKey string
	zoneID          string
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// changeRecord 使用 AWS4-HMAC-SHA256 签名并向 Route 53 提交资源记录变更批次。
func (a *AWSProvider) changeRecord(action, fqdn, value string) error {
	cleanZoneID := strings.TrimPrefix(a.zoneID, "/hostedzone/")
	cleanFQDN := strings.TrimSuffix(fqdn, ".") + "."

	type resourceRecord struct {
		Value string `xml:"Value"`
	}
	type resourceRecordSet struct {
		Name            string           `xml:"Name"`
		Type            string           `xml:"Type"`
		TTL             int              `xml:"TTL"`
		ResourceRecords []resourceRecord `xml:"ResourceRecords>ResourceRecord"`
	}
	type change struct {
		Action            string            `xml:"Action"`
		ResourceRecordSet resourceRecordSet `xml:"ResourceRecordSet"`
	}
	type changeResourceRecordSetsRequest struct {
		XMLName xml.Name `xml:"https://route53.amazonaws.com/doc/2013-04-01/ ChangeResourceRecordSetsRequest"`
		Changes []change `xml:"ChangeBatch>Changes>Change"`
	}

	reqPayload, err := xml.Marshal(changeResourceRecordSetsRequest{
		Changes: []change{{
			Action: action,
			ResourceRecordSet: resourceRecordSet{
				Name:            cleanFQDN,
				Type:            "TXT",
				TTL:             60,
				ResourceRecords: []resourceRecord{{Value: fmt.Sprintf("%q", value)}},
			},
		}},
	})
	if err != nil {
		return err
	}

	host := "route53.amazonaws.com"
	uri := "/2013-04-01/hostedzone/" + cleanZoneID + "/rrset/"
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	payloadHash := sha256Hex(reqPayload)
	canonicalHeaders := "host:" + host + "\nx-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-date"
	canonicalRequest := "POST\n" + uri + "\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash

	scope := dateStamp + "/us-east-1/route53/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))

	kDate := hmacSHA256([]byte("AWS4"+a.secretAccessKey), dateStamp)
	kRegion := hmacSHA256(kDate, "us-east-1")
	kService := hmacSHA256(kRegion, "route53")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	req, err := http.NewRequest(http.MethodPost, "https://"+host+uri, bytes.NewReader(reqPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		a.accessKeyID, scope, signedHeaders, signature,
	))

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("AWS Route53 API error [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Present 在指定 Route 53 Hosted Zone 中创建或更新 TXT 记录。
func (a *AWSProvider) Present(fqdn, value string) error {
	return a.changeRecord("UPSERT", fqdn, value)
}

// CleanUp 从指定 Route 53 Hosted Zone 中删除验证 TXT 记录。
func (a *AWSProvider) CleanUp(fqdn, value string) error {
	return a.changeRecord("DELETE", fqdn, value)
}

func main() {
	provider := &AWSProvider{
		accessKeyID:     os.Getenv("DNS_ID"),
		secretAccessKey: os.Getenv("DNS_SECRET"),
		zoneID:          os.Getenv("DNS_ZONE"),
	}
	if provider.accessKeyID == "" || provider.secretAccessKey == "" || provider.zoneID == "" {
		fmt.Fprintln(os.Stderr, "Error: environment variables DNS_ID, DNS_SECRET, and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

