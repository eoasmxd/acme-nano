package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eoasmxd/acme-nano/core"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// PorkbunProvider 封装 Porkbun API v3 的 TXT 记录增删操作。
type PorkbunProvider struct {
	apiKey       string
	secretAPIKey string
	domain       string
	lastRecordID string
}

func (p *PorkbunProvider) doRequest(endpoint string, payload map[string]any) ([]byte, error) {
	payload["apikey"] = p.apiKey
	payload["secretapikey"] = p.secretAPIKey
	b, _ := json.Marshal(payload)

	resp, err := httpClient.Post("https://api.porkbun.com/api/json/v3/"+endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var status struct {
		Status string `json:"status"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(body, &status) != nil || status.Status != "SUCCESS" {
		return nil, fmt.Errorf("Porkbun DNS API error [%d]: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// Present 在指定 Porkbun 域名下创建一条 TXT 记录。
func (p *PorkbunProvider) Present(fqdn, value string) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(p.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}

	body, err := p.doRequest("dns/create/"+cleanZone, map[string]any{
		"name":    sub,
		"type":    "TXT",
		"content": value,
		"ttl":     "600",
	})
	if err != nil {
		return err
	}
	var res struct {
		ID any `json:"id"`
	}
	if err := json.Unmarshal(body, &res); err != nil || res.ID == nil {
		return fmt.Errorf("failed to parse Porkbun record ID: %s", string(body))
	}
	p.lastRecordID = fmt.Sprintf("%v", res.ID)
	return nil
}

// CleanUp 根据内存中记录的 ID 删除此前创建的 Porkbun TXT 记录。
func (p *PorkbunProvider) CleanUp(fqdn, value string) error {
	if p.lastRecordID == "" {
		return nil
	}
	id := p.lastRecordID
	p.lastRecordID = ""
	cleanZone := strings.ToLower(strings.TrimSuffix(p.domain, "."))
	_, err := p.doRequest(fmt.Sprintf("dns/delete/%s/%s", cleanZone, id), map[string]any{})
	return err
}

func main() {
	provider := &PorkbunProvider{
		apiKey:       os.Getenv("DNS_ID"),
		secretAPIKey: os.Getenv("DNS_SECRET"),
		domain:       os.Getenv("DNS_ZONE"),
	}
	if provider.apiKey == "" || provider.secretAPIKey == "" || provider.domain == "" {
		core.LogError("Error: environment variables DNS_ID, DNS_SECRET, and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

