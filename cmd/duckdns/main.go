package main

import (
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

// DuckDNSProvider 封装 DuckDNS HTTP API 的 TXT 记录设置与清除操作。
type DuckDNSProvider struct {
	token  string
	domain string
}

func (d *DuckDNSProvider) resolveSubDomain(fqdn string) (string, error) {
	target := d.domain
	if target == "" {
		target = fqdn
	}
	clean := strings.ToLower(strings.TrimSuffix(target, "."))
	clean = strings.TrimPrefix(clean, "_acme-challenge.")
	clean = strings.TrimSuffix(clean, ".duckdns.org")
	parts := strings.Split(clean, ".")
	sub := parts[len(parts)-1]
	if sub == "" {
		return "", fmt.Errorf("unable to extract DuckDNS domain from %s", fqdn)
	}
	return sub, nil
}

func (d *DuckDNSProvider) updateTXT(fqdn, value string, clear bool) error {
	sub, err := d.resolveSubDomain(fqdn)
	if err != nil {
		return err
	}
	params := url.Values{
		"domains": {sub},
		"token":   {d.token},
		"txt":     {value},
	}
	if clear {
		params.Set("clear", "true")
	}
	resp, err := httpClient.Get("https://www.duckdns.org/update?" + params.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 || strings.TrimSpace(string(body)) != "OK" {
		return fmt.Errorf("DuckDNS update failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Present 在指定 DuckDNS 域名上设置验证 TXT 记录值。
func (d *DuckDNSProvider) Present(fqdn, value string) error {
	return d.updateTXT(fqdn, value, false)
}

// CleanUp 清除指定 DuckDNS 域名上的验证 TXT 记录值。
func (d *DuckDNSProvider) CleanUp(fqdn, value string) error {
	return d.updateTXT(fqdn, "", true)
}

func main() {
	provider := &DuckDNSProvider{
		token:  os.Getenv("DNS_SECRET"),
		domain: os.Getenv("DNS_ZONE"),
	}
	if provider.token == "" {
		fmt.Fprintln(os.Stderr, "Error: environment variable DNS_SECRET must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

