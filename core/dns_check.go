package core

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"time"
)

// waitForDNSPropagation polls DNS TXT records for fqdn until expectedValue appears or timeout expires.
// waitForDNSPropagation 轮询查询指定 FQDN 的 TXT 记录，确认目标验证值在公网可见。
func waitForDNSPropagation(fqdn, expectedValue string, timeout time.Duration) error {
	cleanFQDN := strings.TrimSuffix(fqdn, ".")
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		records, err := net.LookupTXT(cleanFQDN)
		if err == nil && slices.Contains(records, expectedValue) {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("DNS TXT propagation timeout for %s (%v)", cleanFQDN, timeout)
}
