package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/eoasmxd/acme-nano/core"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// CloudflareProvider implements core.DNSProvider using Cloudflare API v4.
// CloudflareProvider 封装 Cloudflare API v4 的 TXT 记录增删操作。
type CloudflareProvider struct {
	token        string
	zoneID       string
	lastRecordID string
}

// Present creates a TXT record in the configured Cloudflare zone.
// Present 在指定 Cloudflare Zone 中创建一条 TXT 记录。
func (c *CloudflareProvider) Present(fqdn, value string) error {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", c.zoneID)
	payload, _ := json.Marshal(map[string]any{
		"type":    "TXT",
		"name":    fqdn,
		"content": value,
		"ttl":     120,
	})
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var res struct {
		Success bool `json:"success"`
		Result  struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(body, &res) != nil || !res.Success {
		return fmt.Errorf("Cloudflare DNS record creation failed [%d]: %s", resp.StatusCode, string(body))
	}
	c.lastRecordID = res.Result.ID
	return nil
}

// CleanUp deletes the previously created TXT record by its stored record ID.
// CleanUp 根据内存中记录的 ID 删除此前创建的 TXT 记录。
func (c *CloudflareProvider) CleanUp(fqdn, value string) error {
	if c.lastRecordID == "" {
		return nil
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", c.zoneID, c.lastRecordID)
	c.lastRecordID = ""
	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var res struct {
		Success bool `json:"success"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(body, &res) != nil || !res.Success {
		return fmt.Errorf("Cloudflare DNS record deletion failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

func main() {
	provider := &CloudflareProvider{
		token:  os.Getenv("DNS_SECRET"),
		zoneID: os.Getenv("DNS_ZONE"),
	}
	if provider.token == "" || provider.zoneID == "" {
		fmt.Fprintln(os.Stderr, "Error: environment variables DNS_SECRET and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}
