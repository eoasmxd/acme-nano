package main

import (
	"bytes"
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

// HetznerCloudProvider 封装 Hetzner Cloud DNS API v1 的 TXT RRSet 增删操作。
type HetznerCloudProvider struct {
	token  string
	domain string
}

func (h *HetznerCloudProvider) rrsetAction(action, fqdn string, body map[string]any) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(h.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}

	apiURL := fmt.Sprintf(
		"https://api.hetzner.cloud/v1/zones/%s/rrsets/%s/TXT/actions/%s",
		url.PathEscape(cleanZone), url.PathEscape(sub), action,
	)
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		if action == "remove_records" && resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("Hetzner Cloud DNS %s failed [%d]: %s", action, resp.StatusCode, string(respBody))
	}
	return nil
}

// Present 调用 Hetzner Cloud add_records 接口添加验证 TXT 记录。
func (h *HetznerCloudProvider) Present(fqdn, value string) error {
	return h.rrsetAction("add_records", fqdn, map[string]any{
		"ttl": 120,
		"records": []map[string]string{
			{"value": fmt.Sprintf("%q", value)},
		},
	})
}

// CleanUp 调用 Hetzner Cloud remove_records 接口移除验证 TXT 记录。
func (h *HetznerCloudProvider) CleanUp(fqdn, value string) error {
	return h.rrsetAction("remove_records", fqdn, map[string]any{
		"records": []map[string]string{
			{"value": fmt.Sprintf("%q", value)},
		},
	})
}

func main() {
	provider := &HetznerCloudProvider{
		token:  os.Getenv("DNS_SECRET"),
		domain: os.Getenv("DNS_ZONE"),
	}
	if provider.token == "" || provider.domain == "" {
		core.LogError("Error: environment variables DNS_SECRET and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

