package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
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

var ovhEndpoints = map[string]string{
	"ovh-eu":        "https://eu.api.ovh.com/1.0",
	"ovh-us":        "https://api.us.ovhcloud.com/1.0",
	"ovh-ca":        "https://ca.api.ovh.com/1.0",
	"kimsufi-eu":    "https://eu.api.kimsufi.com/1.0",
	"kimsufi-ca":    "https://ca.api.kimsufi.com/1.0",
	"soyoustart-eu": "https://eu.api.soyoustart.com/1.0",
	"soyoustart-ca": "https://ca.api.soyoustart.com/1.0",
}

// OVHProvider 封装 OVHcloud Domain API v1.0 的 TXT 记录增删与刷新操作。
type OVHProvider struct {
	endpoint     string
	appKey       string
	appSecret    string
	consumerKey  string
	domain       string
	lastRecordID int64
}

func (o *OVHProvider) doRequest(method, path string, payload any) ([]byte, error) {
	timeResp, err := httpClient.Get(o.endpoint + "/auth/time")
	if err != nil {
		return nil, err
	}
	timeBytes, _ := io.ReadAll(timeResp.Body)
	timeResp.Body.Close()
	timestamp := strings.TrimSpace(string(timeBytes))

	var bodyStr string
	var bodyReader io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		bodyStr = string(b)
		bodyReader = bytes.NewReader(b)
	}

	fullURL := o.endpoint + path
	toSign := strings.Join([]string{o.appSecret, o.consumerKey, method, fullURL, bodyStr, timestamp}, "+")
	sum := sha1.Sum([]byte(toSign))
	sig := "$1$" + hex.EncodeToString(sum[:])

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Ovh-Application", o.appKey)
	req.Header.Set("X-Ovh-Consumer", o.consumerKey)
	req.Header.Set("X-Ovh-Timestamp", timestamp)
	req.Header.Set("X-Ovh-Signature", sig)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json;charset=utf-8")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OVH API error [%d]: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// Present 在指定 OVH 域名下创建 TXT 记录并触发 Zone 刷新。
func (o *OVHProvider) Present(fqdn, value string) error {
	cleanZone := strings.ToLower(strings.TrimSuffix(o.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	sub, ok := strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}

	body, err := o.doRequest(http.MethodPost, "/domain/zone/"+cleanZone+"/record", map[string]any{
		"fieldType": "TXT",
		"subDomain": sub,
		"target":    value,
		"ttl":       60,
	})
	if err != nil {
		return err
	}
	var res struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &res); err != nil || res.ID == 0 {
		return fmt.Errorf("failed to parse OVH record ID: %s", string(body))
	}
	o.lastRecordID = res.ID
	_, err = o.doRequest(http.MethodPost, "/domain/zone/"+cleanZone+"/refresh", nil)
	return err
}

// CleanUp 根据内存中记录的 ID 删除 OVH TXT 记录并触发 Zone 刷新。
func (o *OVHProvider) CleanUp(fqdn, value string) error {
	if o.lastRecordID == 0 {
		return nil
	}
	id := o.lastRecordID
	o.lastRecordID = 0
	cleanZone := strings.ToLower(strings.TrimSuffix(o.domain, "."))
	if _, err := o.doRequest(http.MethodDelete, fmt.Sprintf("/domain/zone/%s/record/%d", cleanZone, id), nil); err != nil {
		return err
	}
	_, err := o.doRequest(http.MethodPost, "/domain/zone/"+cleanZone+"/refresh", nil)
	return err
}

func main() {
	appKey := os.Getenv("DNS_ID")
	appSecret, consumerKey, okSecret := strings.Cut(os.Getenv("DNS_SECRET"), ":")
	rawZone := os.Getenv("DNS_ZONE")

	endpoint := ovhEndpoints["ovh-eu"]
	domain := rawZone
	if epName, dom, ok := strings.Cut(rawZone, "/"); ok {
		if u, exists := ovhEndpoints[epName]; exists {
			endpoint = u
			domain = dom
		}
	}

	if appKey == "" || !okSecret || appSecret == "" || consumerKey == "" || domain == "" {
		core.LogError("Error: set DNS_ID='AppKey', DNS_SECRET='AppSecret:ConsumerKey', DNS_ZONE='example.com' (or 'ovh-eu/example.com')")
		os.Exit(1)
	}
	core.Run(&OVHProvider{
		endpoint:    endpoint,
		appKey:      appKey,
		appSecret:   appSecret,
		consumerKey: consumerKey,
		domain:      domain,
	})
}

