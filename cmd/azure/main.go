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

// AzureProvider 封装 Microsoft Azure DNS REST API (2018-05-01) 的 TXT 记录增删操作。
type AzureProvider struct {
	tenantID       string
	clientID       string
	clientSecret   string
	subscriptionID string
	resourceGroup  string
	zoneName       string
}

func (a *AzureProvider) getAccessToken() (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", a.tenantID)
	resp, err := httpClient.PostForm(tokenURL, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {a.clientID},
		"client_secret": {a.clientSecret},
		"scope":         {"https://management.azure.com/.default"},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var res struct {
		AccessToken string `json:"access_token"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(body, &res) != nil || res.AccessToken == "" {
		return "", fmt.Errorf("Azure OAuth2 authentication failed [%d]: %s", resp.StatusCode, string(body))
	}
	return res.AccessToken, nil
}

func (a *AzureProvider) recordURI(fqdn string) (string, error) {
	cleanZone := strings.ToLower(strings.TrimSuffix(a.zoneName, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return "", fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}
	return fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnsZones/%s/TXT/%s?api-version=2018-05-01",
		a.subscriptionID, a.resourceGroup, cleanZone, sub,
	), nil
}

// Present 在指定 Azure DNS 区域中创建或更新 TXT 记录集。
func (a *AzureProvider) Present(fqdn, value string) error {
	uri, err := a.recordURI(fqdn)
	if err != nil {
		return err
	}
	token, err := a.getAccessToken()
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"properties": map[string]any{
			"TTL": 60,
			"TXTRecords": []map[string]any{
				{"value": []string{value}},
			},
		},
	})
	req, err := http.NewRequest(http.MethodPut, uri, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Azure DNS PUT failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

// CleanUp 从指定 Azure DNS 区域中删除验证 TXT 记录集。
func (a *AzureProvider) CleanUp(fqdn, value string) error {
	uri, err := a.recordURI(fqdn)
	if err != nil {
		return err
	}
	token, err := a.getAccessToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, uri, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Azure DNS DELETE failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

func main() {
	tenantID, clientID, okID := strings.Cut(os.Getenv("DNS_ID"), "/")
	zoneParts := strings.SplitN(os.Getenv("DNS_ZONE"), "/", 3)
	secret := os.Getenv("DNS_SECRET")
	if !okID || tenantID == "" || clientID == "" || secret == "" || len(zoneParts) != 3 {
		core.LogError("Error: set DNS_ID='tenantID/clientID', DNS_SECRET='clientSecret', DNS_ZONE='subscriptionID/resourceGroup/zoneName'")
		os.Exit(1)
	}
	core.Run(&AzureProvider{
		tenantID:       tenantID,
		clientID:       clientID,
		clientSecret:   secret,
		subscriptionID: zoneParts[0],
		resourceGroup:  zoneParts[1],
		zoneName:       zoneParts[2],
	})
}

