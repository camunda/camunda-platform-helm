//go:build !windows

package matrix

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func processMatches(pid int, identity string) bool {
	err := syscall.Kill(pid, 0)
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	return identity != "" && processIdentity(pid) == identity
}

func processIdentity(pid int) string {
	output, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
