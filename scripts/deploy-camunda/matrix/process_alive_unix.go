//go:build !windows

package matrix

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func processState(pid int, identity string) (bool, bool) {
	err := syscall.Kill(pid, 0)
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false, true
	}
	observed := processIdentity(pid)
	if identity == "" || observed == "" {
		return false, false
	}
	return observed == identity, true
}

func processIdentity(pid int) string {
	output, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
