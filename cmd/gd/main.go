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

// GoDaddyProvider 封装 GoDaddy Domains API v1 的 TXT 记录增删操作。
type GoDaddyProvider struct {
	apiKey    string
	apiSecret string
	domain    string
}

func (g *GoDaddyProvider) splitSubDomain(fqdn string) (string, string, error) {
	cleanZone := strings.ToLower(strings.TrimSuffix(g.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return "", "", fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}
	return cleanZone, sub, nil
}

func (g *GoDaddyProvider) doRequest(method, path string, bodyData any) error {
	var bodyReader io.Reader
	if bodyData != nil {
		b, _ := json.Marshal(bodyData)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "https://api.godaddy.com/v1/"+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", g.apiKey, g.apiSecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("GoDaddy DNS API error [%d]: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// Present 在指定 GoDaddy 域名下设置 TXT 记录。
func (g *GoDaddyProvider) Present(fqdn, value string) error {
	zone, sub, err := g.splitSubDomain(fqdn)
	if err != nil {
		return err
	}
	payload := []map[string]any{
		{"data": value, "ttl": 600},
	}
	return g.doRequest(http.MethodPut, fmt.Sprintf("domains/%s/records/TXT/%s", zone, sub), payload)
}

// CleanUp 删除指定 GoDaddy 域名下的验证 TXT 记录。
func (g *GoDaddyProvider) CleanUp(fqdn, value string) error {
	zone, sub, err := g.splitSubDomain(fqdn)
	if err != nil {
		return err
	}
	return g.doRequest(http.MethodDelete, fmt.Sprintf("domains/%s/records/TXT/%s", zone, sub), nil)
}

func main() {
	provider := &GoDaddyProvider{
		apiKey:    os.Getenv("DNS_ID"),
		apiSecret: os.Getenv("DNS_SECRET"),
		domain:    os.Getenv("DNS_ZONE"),
	}
	if provider.apiKey == "" || provider.apiSecret == "" || provider.domain == "" {
		fmt.Fprintln(os.Stderr, "Error: environment variables DNS_ID, DNS_SECRET, and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

