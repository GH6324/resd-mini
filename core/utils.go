package core

import (
	"fmt"
	"github.com/ncruces/zenity"
	"os/exec"
	"runtime"
)

func DialogErr(message string) {
	err := zenity.Warning(message,
		zenity.Title("Warning"),
		zenity.WarningIcon)
	if err != nil {
		fmt.Println("DialogErr:", err)
	}
}

func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}
