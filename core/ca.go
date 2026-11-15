package core

import (
	"fmt"
	"strings"
)

// Built-in ACME v2 CA directory URL constants.
// 内置主流 ACME v2 发证机构 Directory URL 常量。
const (
	CALetsEncrypt        = "https://acme-v02.api.letsencrypt.org/directory"
	CALetsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
	CABuypass            = "https://api.buypass.com/acme/directory"
	CABuypassStaging     = "https://api.test4.buypass.no/acme/directory"
	CAZeroSSL            = "https://acme.zerossl.com/v2/DV90"
	CAGoogle             = "https://dv.acme-v02.api.pki.goog/directory"
	CAGoogleStaging      = "https://dv.acme-v02.test-api.pki.goog/directory"
)

var directoryPresets = map[string]string{
	"letsencrypt":         CALetsEncrypt,
	"letsencrypt-staging": CALetsEncryptStaging,
	"buypass":             CABuypass,
	"buypass-staging":     CABuypassStaging,
	"zerossl":             CAZeroSSL,
	"google":              CAGoogle,
	"google-staging":      CAGoogleStaging,
}

// ResolveDirectoryURL resolves a preset CA name or custom directory URL into an ACME directory endpoint.
// ResolveDirectoryURL 将预设 CA 名称或自定义目录 URL 解析为有效的 ACME Directory 地址。
func ResolveDirectoryURL(caOrURL string) (string, error) {
	target := strings.TrimSpace(caOrURL)
	if target == "" {
		return CALetsEncrypt, nil
	}
	if dirURL, ok := directoryPresets[strings.ToLower(target)]; ok {
		return dirURL, nil
	}
	if strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://") {
		return target, nil
	}
	return "", fmt.Errorf("unknown CA preset or invalid directory URL: %s", caOrURL)
}

