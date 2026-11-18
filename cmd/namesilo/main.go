package main

import (
	"encoding/xml"
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

// NameSiloProvider 封装 NameSilo XML API 的 TXT 记录增删操作。
type NameSiloProvider struct {
	apiKey       string
	domain       string
	lastRecordID string
}

type nameSiloReply struct {
	Code     int    `xml:"reply>code"`
	Detail   string `xml:"reply>detail"`
	RecordID string `xml:"reply>record_id"`
}

func (n *NameSiloProvider) doRequest(operation string, params url.Values) (*nameSiloReply, error) {
	params.Set("version", "1")
	params.Set("type", "xml")
	params.Set("key", n.apiKey)

	resp, err := httpClient.Get("https://www.namesilo.com/api/" + operation + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var parsed nameSiloReply
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse NameSilo XML: %s", string(body))
	}
	if parsed.Code != 300 {
		return nil, fmt.Errorf("NameSilo API error [%d]: %s", parsed.Code, parsed.Detail)
	}
	return &parsed, nil
}

// Present 在指定 NameSilo 域名下添加一条 TXT 记录。
func (n *NameSiloProvider) Present(fqdn, value string) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(n.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}

	reply, err := n.doRequest("dnsAddRecord", url.Values{
		"domain":  {cleanZone},
		"rrtype":  {"TXT"},
		"rrhost":  {sub},
		"rrvalue": {value},
		"rrttl":   {"3600"},
	})
	if err != nil {
		return err
	}
	n.lastRecordID = reply.RecordID
	return nil
}

// CleanUp 根据内存中记录的 rrid 删除此前创建的 NameSilo TXT 记录。
func (n *NameSiloProvider) CleanUp(fqdn, value string) error {
	if n.lastRecordID == "" {
		return nil
	}
	id := n.lastRecordID
	n.lastRecordID = ""
	cleanZone := strings.ToLower(strings.TrimSuffix(n.domain, "."))
	_, err := n.doRequest("dnsDeleteRecord", url.Values{
		"domain": {cleanZone},
		"rrid":   {id},
	})
	return err
}

func main() {
	provider := &NameSiloProvider{
		apiKey: os.Getenv("DNS_SECRET"),
		domain: os.Getenv("DNS_ZONE"),
	}
	if provider.apiKey == "" || provider.domain == "" {
		core.LogError("Error: environment variables DNS_SECRET and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

