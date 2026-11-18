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

// DigitalOceanProvider 封装 DigitalOcean API v2 的 TXT 记录增删操作。
type DigitalOceanProvider struct {
	token        string
	domain       string
	lastRecordID int64
}

func (d *DigitalOceanProvider) deleteRecord(zone string, recordID int64) error {
	apiURL := fmt.Sprintf("https://api.digitalocean.com/v2/domains/%s/records/%d", zone, recordID)
	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+d.token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DigitalOcean DNS delete failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Present 在指定 DigitalOcean 域名下创建一条 TXT 记录。
func (d *DigitalOceanProvider) Present(fqdn, value string) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(d.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}

	apiURL := fmt.Sprintf("https://api.digitalocean.com/v2/domains/%s/records", cleanZone)
	payload, _ := json.Marshal(map[string]any{
		"type": "TXT",
		"name": sub,
		"data": value,
		"ttl":  120,
	})
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var res struct {
		DomainRecord struct {
			ID int64 `json:"id"`
		} `json:"domain_record"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(body, &res) != nil || res.DomainRecord.ID == 0 {
		return fmt.Errorf("DigitalOcean DNS create failed [%d]: %s", resp.StatusCode, string(body))
	}
	d.lastRecordID = res.DomainRecord.ID
	return nil
}

// CleanUp 根据内存中记录的 ID 删除此前创建的 TXT 记录。
func (d *DigitalOceanProvider) CleanUp(fqdn, value string) error {
	if d.lastRecordID == 0 {
		return nil
	}
	id := d.lastRecordID
	d.lastRecordID = 0
	cleanZone := strings.ToLower(strings.TrimSuffix(d.domain, "."))
	return d.deleteRecord(cleanZone, id)
}

func main() {
	provider := &DigitalOceanProvider{
		token:  os.Getenv("DNS_SECRET"),
		domain: os.Getenv("DNS_ZONE"),
	}
	if provider.token == "" || provider.domain == "" {
		core.LogError("Error: environment variables DNS_SECRET and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

