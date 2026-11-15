package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eoasmxd/acme-nano/core"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// NamecheapProvider 封装 Namecheap XML API 的 TXT 记录增删操作。
type NamecheapProvider struct {
	apiUser  string
	clientIP string
	apiKey   string
	domain   string
}

type ncHost struct {
	Name    string `xml:"Name,attr"`
	Type    string `xml:"Type,attr"`
	Address string `xml:"Address,attr"`
	MXPref  string `xml:"MXPref,attr"`
	TTL     string `xml:"TTL,attr"`
}

type ncResponse struct {
	Status string `xml:"Status,attr"`
	Errors []struct {
		Message string `xml:",chardata"`
	} `xml:"Errors>Error"`
	HostsResult *struct {
		Hosts []ncHost `xml:"host"`
	} `xml:"CommandResponse>DomainDNSGetHostsResult"`
}

func (n *NamecheapProvider) splitDomain(fqdn string) (sld, tld, sub string, err error) {
	cleanZone := strings.ToLower(strings.TrimSuffix(n.domain, "."))
	cleanFQDN := strings.ToLower(strings.TrimSuffix(fqdn, "."))
	var ok bool
	sub, ok = strings.CutSuffix(cleanFQDN, "."+cleanZone)
	if !ok || sub == "" {
		return "", "", "", fmt.Errorf("domain %s does not belong to zone %s", cleanFQDN, cleanZone)
	}
	sld, tld, ok = strings.Cut(cleanZone, ".")
	if !ok || sld == "" || tld == "" {
		return "", "", "", fmt.Errorf("invalid Namecheap zone %s", cleanZone)
	}
	return sld, tld, sub, nil
}

func (n *NamecheapProvider) postCommand(command string, extra url.Values) (*ncResponse, error) {
	vals := url.Values{}
	vals.Set("ApiUser", n.apiUser)
	vals.Set("ApiKey", n.apiKey)
	vals.Set("UserName", n.apiUser)
	vals.Set("ClientIp", n.clientIP)
	vals.Set("Command", command)
	for k, v := range extra {
		vals[k] = v
	}

	resp, err := httpClient.PostForm("https://api.namecheap.com/xml.response", vals)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var parsed ncResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse Namecheap XML response: %s", string(body))
	}
	if parsed.Status != "OK" {
		if len(parsed.Errors) > 0 {
			return nil, fmt.Errorf("Namecheap API error: %s", parsed.Errors[0].Message)
		}
		return nil, fmt.Errorf("Namecheap API error: %s", string(body))
	}
	return &parsed, nil
}

func (n *NamecheapProvider) setHosts(sld, tld string, hosts []ncHost) error {
	params := url.Values{}
	params.Set("SLD", sld)
	params.Set("TLD", tld)
	for i, h := range hosts {
		idx := strconv.Itoa(i + 1)
		params.Set("HostName"+idx, h.Name)
		params.Set("RecordType"+idx, h.Type)
		params.Set("Address"+idx, h.Address)
		if h.MXPref != "" {
			params.Set("MXPref"+idx, h.MXPref)
		}
		if h.TTL != "" {
			params.Set("TTL"+idx, h.TTL)
		}
	}
	_, err := n.postCommand("namecheap.domains.dns.setHosts", params)
	return err
}

// Present 读取 Namecheap 域名现有解析记录并追加验证 TXT 记录后写回。
func (n *NamecheapProvider) Present(fqdn, value string) error {
	sld, tld, sub, err := n.splitDomain(fqdn)
	if err != nil {
		return err
	}
	res, err := n.postCommand("namecheap.domains.dns.getHosts", url.Values{"SLD": {sld}, "TLD": {tld}})
	if err != nil {
		return err
	}
	if res.HostsResult == nil {
		return fmt.Errorf("Namecheap getHosts returned empty result block")
	}
	hosts := append(res.HostsResult.Hosts, ncHost{
		Name:    sub,
		Type:    "TXT",
		Address: value,
		MXPref:  "10",
		TTL:     "120",
	})
	return n.setHosts(sld, tld, hosts)
}

// CleanUp 读取 Namecheap 域名现有解析记录，过滤掉验证 TXT 记录后写回。
func (n *NamecheapProvider) CleanUp(fqdn, value string) error {
	sld, tld, sub, err := n.splitDomain(fqdn)
	if err != nil {
		return err
	}
	res, err := n.postCommand("namecheap.domains.dns.getHosts", url.Values{"SLD": {sld}, "TLD": {tld}})
	if err != nil || res.HostsResult == nil {
		return err
	}
	var filtered []ncHost
	for _, h := range res.HostsResult.Hosts {
		if h.Type == "TXT" && strings.EqualFold(h.Name, sub) && h.Address == value {
			continue
		}
		filtered = append(filtered, h)
	}
	if len(filtered) == len(res.HostsResult.Hosts) {
		return nil
	}
	return n.setHosts(sld, tld, filtered)
}

func main() {
	user, clientIP, _ := strings.Cut(os.Getenv("DNS_ID"), ":")
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}
	provider := &NamecheapProvider{
		apiUser:  user,
		clientIP: clientIP,
		apiKey:   os.Getenv("DNS_SECRET"),
		domain:   os.Getenv("DNS_ZONE"),
	}
	if provider.apiUser == "" || provider.apiKey == "" || provider.domain == "" {
		fmt.Fprintln(os.Stderr, "Error: environment variables DNS_ID (ApiUser:ClientIp), DNS_SECRET (ApiKey), and DNS_ZONE must be set")
		os.Exit(1)
	}
	core.Run(provider)
}

