package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/eoasmxd/acme-nano/core"
)

// ManualProvider prompts the user via stdin/stdout to manually create and remove DNS TXT records.
// ManualProvider 通过终端交互提示用户手动添加与删除 DNS TXT 记录。
type ManualProvider struct{}

// Present displays the TXT record details and waits for the user to press Enter.
// Present 打印 TXT 记录详情并等待用户按回车键继续。
func (m *ManualProvider) Present(fqdn, value string) error {
	fmt.Printf("\n[Manual DNS Challenge] Add the following TXT record at your DNS provider:\n")
	fmt.Printf("  FQDN : %s\n", fqdn)
	fmt.Printf("  Value: %s\n", value)
	fmt.Printf("Press Enter after the record has been added...")
	_, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return err
}

// CleanUp notifies the user that the challenge TXT record can be safely removed.
// CleanUp 提示用户验证已完成，可删除对应的 TXT 记录。
func (m *ManualProvider) CleanUp(fqdn, value string) error {
	fmt.Printf("[Manual DNS CleanUp] Validation finished. You may now remove %s (value: %s)\n", fqdn, value)
	return nil
}

func main() {
	core.Run(&ManualProvider{})
}
