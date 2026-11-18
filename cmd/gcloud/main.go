package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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

// GCloudProvider 封装 Google Cloud DNS REST API v1 的 TXT 记录增删操作。
type GCloudProvider struct {
	projectID string
	secret    string
	zoneName  string
}

// getAccessToken 获取 OAuth2 访问令牌，支持直接 Bearer Token 或 Service Account JSON 密钥。
func (g *GCloudProvider) getAccessToken() (string, error) {
	trimmed := strings.TrimSpace(g.secret)
	if !strings.HasPrefix(trimmed, "{") {
		return trimmed, nil
	}
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal([]byte(trimmed), &sa); err != nil {
		return "", fmt.Errorf("invalid GCP service account JSON: %w", err)
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode service account PEM private key")
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	rsaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("service account key is not RSA")
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	now := time.Now().Unix()
	headerJSON, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claimJSON, _ := json.Marshal(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/ndev.clouddns.readwrite",
		"aud":   tokenURI,
		"iat":   now,
		"exp":   now + 3600,
	})
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimJSON)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	jwt := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	resp, err := httpClient.PostForm(tokenURI, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwt},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tokenRes struct {
		AccessToken string `json:"access_token"`
	}
	if resp.StatusCode >= 300 || json.Unmarshal(body, &tokenRes) != nil || tokenRes.AccessToken == "" {
		return "", fmt.Errorf("GCP OAuth2 token exchange failed [%d]: %s", resp.StatusCode, string(body))
	}
	return tokenRes.AccessToken, nil
}

// Present 在指定 Google Cloud DNS Managed Zone 中创建 TXT 记录集。
func (g *GCloudProvider) Present(fqdn, value string) error {
	token, err := g.getAccessToken()
	if err != nil {
		return err
	}
	dnsName := strings.TrimSuffix(fqdn, ".") + "."
	_ = g.deleteRRSet(token, dnsName)

	apiURL := fmt.Sprintf("https://dns.googleapis.com/dns/v1/projects/%s/managedZones/%s/rrsets", g.projectID, g.zoneName)
	payload, _ := json.Marshal(map[string]any{
		"name":    dnsName,
		"type":    "TXT",
		"ttl":     60,
		"rrdatas": []string{fmt.Sprintf("%q", value)},
	})
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
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
		return fmt.Errorf("Google Cloud DNS create failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (g *GCloudProvider) deleteRRSet(token, dnsName string) error {
	apiURL := fmt.Sprintf("https://dns.googleapis.com/dns/v1/projects/%s/managedZones/%s/rrsets/%s/TXT", g.projectID, g.zoneName, dnsName)
	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
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
		return fmt.Errorf("Google Cloud DNS delete failed [%d]: %s", resp.StatusCode, string(body))
	}
	return nil
}

// CleanUp 从指定 Google Cloud DNS Managed Zone 中删除验证 TXT 记录集。
func (g *GCloudProvider) CleanUp(fqdn, value string) error {
	token, err := g.getAccessToken()
	if err != nil {
		return err
	}
	dnsName := strings.TrimSuffix(fqdn, ".") + "."
	return g.deleteRRSet(token, dnsName)
}

func main() {
	provider := &GCloudProvider{
		projectID: os.Getenv("DNS_ID"),
		secret:    os.Getenv("DNS_SECRET"),
		zoneName:  os.Getenv("DNS_ZONE"),
	}
	if provider.projectID == "" || provider.secret == "" || provider.zoneName == "" {
		core.LogError("Error: environment variables DNS_ID (Project ID), DNS_SECRET (Bearer Token or SA JSON), and DNS_ZONE (Managed Zone) must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

