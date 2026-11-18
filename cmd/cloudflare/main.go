package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// deleteRecord deletes a DNS record in the Cloudflare zone by its record ID.
// deleteRecord 根据记录 ID 删除 Cloudflare Zone 中的指定 DNS 记录。
func (c *CloudflareProvider) deleteRecord(recordID string) error {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", c.zoneID, recordID)
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

// Present creates a TXT record in the configured Cloudflare zone.
// Present 在指定 Cloudflare Zone 中创建一条 TXT 记录。
func (c *CloudflareProvider) Present(fqdn, value string) error {
	listURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=TXT&name=%s", c.zoneID, url.QueryEscape(fqdn))
	listReq, err := http.NewRequest(http.MethodGet, listURL, nil)
	if err != nil {
		return err
	}
	listReq.Header.Set("Authorization", "Bearer "+c.token)

	listResp, err := httpClient.Do(listReq)
	if err != nil {
		return err
	}
	listBody, _ := io.ReadAll(listResp.Body)
	listResp.Body.Close()

	var listRes struct {
		Success bool `json:"success"`
		Result  []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if listResp.StatusCode >= 300 || json.Unmarshal(listBody, &listRes) != nil || !listRes.Success {
		return fmt.Errorf("Cloudflare DNS record query failed [%d]: %s", listResp.StatusCode, string(listBody))
	}
	for _, rec := range listRes.Result {
		if err := c.deleteRecord(rec.ID); err != nil {
			return err
		}
	}

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
	id := c.lastRecordID
	c.lastRecordID = ""
	return c.deleteRecord(id)
}

func main() {
	provider := &CloudflareProvider{
		token:  os.Getenv("DNS_SECRET"),
		zoneID: os.Getenv("DNS_ZONE"),
	}
	if provider.token == "" || provider.zoneID == "" {
		core.LogError("Error: environment variables DNS_SECRET and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}
