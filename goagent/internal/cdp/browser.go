package cdp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// FindBrowserPath 按候选顺序寻找本机的 Edge/Chrome。
func FindBrowserPath(explicit string) (string, error) {
	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		candidates = append(candidates,
			pf+`\Microsoft\Edge\Application\msedge.exe`,
			pf+`\Google\Chrome\Application\chrome.exe`,
		)
	}
	if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
		candidates = append(candidates,
			pf86+`\Microsoft\Edge\Application\msedge.exe`,
			pf86+`\Google\Chrome\Application\chrome.exe`,
		)
	}
	if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
		candidates = append(candidates, lad+`\Google\Chrome\Application\chrome.exe`)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("未找到 Edge/Chrome，可用 --browser 指定路径")
}

// PortAlive 判断调试端口是否已被占用（占用即认为浏览器已在运行）。
func PortAlive(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Launch 以调试模式启动浏览器并等待调试端口就绪。端口已占用时直接复用。
func Launch(ctx context.Context, browserPath, url string, port int, userDataDir string) error {
	if PortAlive(port) {
		return nil
	}
	if browserPath == "" {
		var err error
		browserPath, err = FindBrowserPath("")
		if err != nil {
			return err
		}
	}
	if userDataDir == "" {
		userDataDir = filepath.Join(os.TempDir(), "eagent-cli-profile")
	}
	args := []string{
		"--remote-debugging-port=" + fmt.Sprint(port),
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-popup-blocking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--remote-allow-origins=*",
		"--disable-features=CalculateNativeWinOcclusion",
		"--disable-blink-features=AutomationControlled",
		"--test-type",
		"--app=" + url,
	}
	cmd := exec.Command(browserPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动浏览器失败: %w", err)
	}
	go func() { _ = cmd.Wait() }()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if PortAlive(port) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return errors.New("调试端口 20 秒内未就绪")
}
