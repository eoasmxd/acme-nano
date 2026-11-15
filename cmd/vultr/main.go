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

// VultrProvider 封装 Vultr API v2 的 TXT 记录增删操作。
type VultrProvider struct {
	apiKey       string
	domain       string
	lastRecordID string
}

// Present 在指定 Vultr 域名下创建一条 TXT 记录。
func (v *VultrProvider) Present(fqdn, value string) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(v.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}

	apiURL := fmt.Sprintf("https://api.vultr.com/v2/domains/%s/records", cleanZone)
	payload, _ := json.Marshal(map[string]any{
		"name": sub,
		"type": "TXT",
		"data": value,
		"ttl":  120,
	})
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var res struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(body, &res) != nil || res.Record.ID == "" {
		return fmt.Errorf("Vultr DNS create failed [%d]: %s", resp.StatusCode, string(body))
	}
	v.lastRecordID = res.Record.ID
	return nil
}

// CleanUp 根据内存中记录的 ID 删除此前创建的 Vultr TXT 记录。
func (v *VultrProvider) CleanUp(fqdn, value string) error {
	if v.lastRecordID == "" {
		return nil
	}
	id := v.lastRecordID
	v.lastRecordID = ""
	cleanZone := strings.ToLower(strings.TrimSuffix(v.domain, "."))

	apiURL := fmt.Sprintf("https://api.vultr.com/v2/domains/%s/records/%s", cleanZone, id)
	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Vultr DNS delete failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

func main() {
	provider := &VultrProvider{
		apiKey: os.Getenv("DNS_SECRET"),
		domain: os.Getenv("DNS_ZONE"),
	}
	if provider.apiKey == "" || provider.domain == "" {
		fmt.Fprintln(os.Stderr, "Error: environment variables DNS_SECRET and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

