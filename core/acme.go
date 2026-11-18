package core

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

// DNSProvider defines the callback interface required by the ACME core for DNS-01 challenges.
// DNSProvider 定义内核对外部 DNS 驱动的回调接口契约。
type DNSProvider interface {
	// Present creates a DNS TXT record at the provider.
	// Present 在 DNS 服务商处添加一条 TXT 记录。
	Present(fqdn, value string) error

	// CleanUp removes the DNS TXT record after verification completes.
	// CleanUp 在验证结束后清理该 TXT 记录。
	CleanUp(fqdn, value string) error
}

// Config holds all parameters needed to issue a certificate via ACME DNS-01.
// Config 定义证书签发所需的全部参数。
type Config struct {
	DirectoryURL string
	AccountKey   string
	Domains      []string
	CertOut      string
	KeyOut       string
	EABKID       string
	EABHMAC      string
	Provider     DNSProvider
}

type directoryResponse struct {
	NewNonce   string `json:"newNonce"`
	NewAccount string `json:"newAccount"`
	NewOrder   string `json:"newOrder"`
}

type orderResponse struct {
	Status         string   `json:"status"`
	Authorizations []string `json:"authorizations"`
	Finalize       string   `json:"finalize"`
	Certificate    string   `json:"certificate"`
}

type authorizationResponse struct {
	Status     string `json:"status"`
	Identifier struct {
		Value string `json:"value"`
	} `json:"identifier"`
	Challenges []struct {
		Type  string `json:"type"`
		URL   string `json:"url"`
		Token string `json:"token"`
	} `json:"challenges"`
}

// Run parses standard command-line flags and executes the ACME issuance workflow with the given DNSProvider.
// Run 解析标准命令行参数并使用指定的 DNSProvider 执行证书签发流程。
func Run(provider DNSProvider) {
	domains := flag.String("d", "", "Comma-separated list of domains (e.g. example.com,*.example.com)")
	ca := flag.String("ca", "letsencrypt", "CA preset name (letsencrypt, letsencrypt-staging, buypass, zerossl, google) or directory URL")
	accountKey := flag.String("account", "account.key", "Path to ACME account ECDSA private key")
	certOut := flag.String("cert", "fullchain.pem", "Output path for fullchain certificate PEM")
	keyOut := flag.String("key", "privkey.pem", "Output path for certificate private key PEM")
	showVersion := flag.Bool("v", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(Version())
		return
	}

	err := Issue(Config{
		DirectoryURL: *ca,
		AccountKey:   *accountKey,
		Domains:      strings.Split(*domains, ","),
		CertOut:      *certOut,
		KeyOut:       *keyOut,
		EABKID:       os.Getenv("EAB_KID"),
		EABHMAC:      os.Getenv("EAB_HMAC"),
		Provider:     provider,
	})
	if err != nil {
		LogError("Issuance failed: %v", err)
		os.Exit(1)
	}
}

// Issue executes the complete ACME DNS-01 certificate issuance workflow.
// Issue 执行完整的 ACME DNS-01 证书签发流程。
func Issue(cfg Config) error {
	if cfg.Provider == nil {
		return fmt.Errorf("DNSProvider must not be nil")
	}
	dirURL, err := ResolveDirectoryURL(cfg.DirectoryURL)
	if err != nil {
		return err
	}
	if cfg.AccountKey == "" {
		cfg.AccountKey = "account.key"
	}
	if cfg.CertOut == "" {
		cfg.CertOut = "fullchain.pem"
	}
	if cfg.KeyOut == "" {
		cfg.KeyOut = "privkey.pem"
	}

	var domains []string
	for _, d := range cfg.Domains {
		trimmed := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(d), "."))
		if trimmed != "" && !slices.Contains(domains, trimmed) {
			domains = append(domains, trimmed)
		}
	}
	if len(domains) == 0 {
		return fmt.Errorf("domain list must not be empty")
	}

	if isCertificateStillValid(cfg.CertOut, domains) {
		LogInfo("Local certificate is valid for more than 30 days and covers all domains; skipping issuance.")
		return nil
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	dirResp, err := httpClient.Get(dirURL)
	if err != nil {
		return fmt.Errorf("failed to fetch ACME directory: %w", err)
	}
	var dir directoryResponse
	err = json.NewDecoder(dirResp.Body).Decode(&dir)
	dirResp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to decode ACME directory: %w", err)
	}

	accountKey, err := loadOrCreateECKey(cfg.AccountKey)
	if err != nil {
		return fmt.Errorf("failed to load or generate account key: %w", err)
	}

	client := &jwsClient{
		httpClient: httpClient,
		privateKey: accountKey,
		nonceURL:   dir.NewNonce,
	}

	acctPayload := map[string]any{
		"termsOfServiceAgreed": true,
	}
	if cfg.EABKID != "" || cfg.EABHMAC != "" {
		if cfg.EABKID == "" || cfg.EABHMAC == "" {
			return fmt.Errorf("both EAB KID and EAB HMAC key must be provided together")
		}
		eab, err := buildExternalAccountBinding(cfg.EABKID, cfg.EABHMAC, dir.NewAccount, &accountKey.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to build EAB: %w", err)
		}
		acctPayload["externalAccountBinding"] = eab
	}

	_, accountHeader, err := client.postJWS(dir.NewAccount, acctPayload)
	if err != nil {
		return fmt.Errorf("failed to register or fetch ACME account: %w", err)
	}
	client.kid = accountHeader.Get("Location")

	identifiers := make([]map[string]string, len(domains))
	for i, d := range domains {
		identifiers[i] = map[string]string{"type": "dns", "value": d}
	}
	orderBody, orderHeader, err := client.postJWS(dir.NewOrder, map[string]any{
		"identifiers": identifiers,
	})
	if err != nil {
		return fmt.Errorf("failed to create ACME order: %w", err)
	}
	orderURL := orderHeader.Get("Location")

	var order orderResponse
	if err := json.Unmarshal(orderBody, &order); err != nil {
		return err
	}

	for _, authzURL := range order.Authorizations {
		if err := solveAuthorization(client, authzURL, cfg.Provider); err != nil {
			return err
		}
	}

	certKey, err := loadOrCreateECKey(cfg.KeyOut)
	if err != nil {
		return fmt.Errorf("failed to load or generate certificate key: %w", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		DNSNames: domains,
	}, certKey)
	if err != nil {
		return fmt.Errorf("failed to create CSR: %w", err)
	}

	var finalizeBody []byte
	var finalizeHeader http.Header
	for attempt := 0; attempt < 10; attempt++ {
		finalizeBody, finalizeHeader, err = client.postJWS(order.Finalize, map[string]string{
			"csr": base64.RawURLEncoding.EncodeToString(csrDER),
		})
		if err == nil {
			break
		}
		if attempt < 9 && strings.Contains(err.Error(), "orderNotReady") {
			time.Sleep(3 * time.Second)
			continue
		}
		return fmt.Errorf("failed to finalize ACME order: %w", err)
	}
	if orderURL == "" {
		orderURL = finalizeHeader.Get("Location")
	}
	if err := json.Unmarshal(finalizeBody, &order); err != nil {
		return err
	}

	for i := 0; i < 30 && order.Status != "valid"; i++ {
		if order.Status != "pending" && order.Status != "ready" && order.Status != "processing" {
			return fmt.Errorf("unexpected ACME order status: %s", order.Status)
		}
		if orderURL == "" {
			return fmt.Errorf("missing ACME order URL for status polling")
		}
		time.Sleep(3 * time.Second)
		pollBody, _, err := client.postJWS(orderURL, nil)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(pollBody, &order); err != nil {
			return err
		}
	}
	if order.Certificate == "" {
		return fmt.Errorf("timed out waiting for certificate issuance (status: %s)", order.Status)
	}

	certPEM, _, err := client.postJWS(order.Certificate, nil)
	if err != nil {
		return fmt.Errorf("failed to download certificate chain: %w", err)
	}
	if err := os.WriteFile(cfg.CertOut, certPEM, 0644); err != nil {
		return err
	}
	LogInfo("Certificate issued successfully (%s).", cfg.CertOut)
	return nil
}

// solveAuthorization performs serial DNS-01 provisioning, propagation polling, CA verification, and cleanup.
// solveAuthorization 针对单个域名授权执行串行 DNS-01 挑战、传播探测、CA 轮询与清理。
func solveAuthorization(client *jwsClient, authzURL string, provider DNSProvider) error {
	body, _, err := client.postJWS(authzURL, nil)
	if err != nil {
		return err
	}
	var authz authorizationResponse
	if err := json.Unmarshal(body, &authz); err != nil {
		return err
	}
	if authz.Status == "valid" {
		return nil
	}
	if authz.Status != "pending" {
		return fmt.Errorf("unexpected authorization status for %s: %s", authz.Identifier.Value, authz.Status)
	}

	var challengeURL, token string
	for _, ch := range authz.Challenges {
		if ch.Type == "dns-01" {
			challengeURL = ch.URL
			token = ch.Token
			break
		}
	}
	if challengeURL == "" {
		return fmt.Errorf("no dns-01 challenge found for %s", authz.Identifier.Value)
	}

	fqdn := "_acme-challenge." + strings.TrimPrefix(authz.Identifier.Value, "*.")
	txtValue, err := dns01ChallengeValue(token, &client.privateKey.PublicKey)
	if err != nil {
		return err
	}

	if err := provider.Present(fqdn, txtValue); err != nil {
		return fmt.Errorf("failed to present DNS TXT record (%s): %w", fqdn, err)
	}
	defer func() {
		if cleanErr := provider.CleanUp(fqdn, txtValue); cleanErr != nil {
			LogError("Warning: failed to clean up DNS TXT record (%s): %v", fqdn, cleanErr)
		}
	}()

	if err := waitForDNSPropagation(fqdn, txtValue, 120*time.Second); err != nil {
		LogError("Warning: %v; proceeding to trigger CA validation", err)
	}

	if _, _, err := client.postJWS(challengeURL, map[string]any{}); err != nil {
		return fmt.Errorf("failed to trigger challenge validation (%s): %w", fqdn, err)
	}

	for i := 0; i < 30; i++ {
		time.Sleep(3 * time.Second)
		pollBody, _, err := client.postJWS(authzURL, nil)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(pollBody, &authz); err != nil {
			return err
		}
		if authz.Status == "valid" {
			return nil
		}
		if authz.Status == "invalid" {
			return fmt.Errorf("CA validation failed for %s: %s", authz.Identifier.Value, string(pollBody))
		}
	}
	return fmt.Errorf("timed out waiting for CA validation for %s", authz.Identifier.Value)
}

// isCertificateStillValid checks whether certPath exists, covers all domains, and has >30 days remaining.
// isCertificateStillValid 检查本地证书是否存在、包含所有目标域名且剩余有效期大于 30 天。
func isCertificateStillValid(certPath string, domains []string) bool {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	remaining := time.Until(cert.NotAfter)
	daysLeft := int(remaining.Hours() / 24)
	LogInfo(
		"Found local certificate (%s):\n  Issuer : %s\n  Domains: %s\n  Expires: %s (%d days left)",
		certPath, cert.Issuer.CommonName, strings.Join(cert.DNSNames, ", "), cert.NotAfter.Format(time.RFC3339), daysLeft,
	)

	if remaining <= 30*24*time.Hour {
		return false
	}
	for _, d := range domains {
		if !slices.ContainsFunc(cert.DNSNames, func(san string) bool {
			return strings.EqualFold(san, d)
		}) {
			return false
		}
	}
	return true
}

// LogInfo 格式化并向标准输出打印带统一前缀与换行隔离的日志信息。
func LogInfo(format string, args ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\r\n")
	fmt.Fprintf(os.Stdout, "\n[acme-nano] %s\n", msg)
}

// LogError 格式化并向标准错误输出打印带统一前缀与换行隔离的错误或警告信息。
func LogError(format string, args ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\r\n")
	fmt.Fprintf(os.Stderr, "\n[acme-nano] %s\n", msg)
}
