//go:build windows

package matrix

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func processState(pid int, identity string) (bool, bool) {
	observed := processIdentity(pid)
	if identity == "" || observed == "" {
		return false, false
	}
	return observed == identity, true
}

func processIdentity(pid int) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle) //nolint:errcheck
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", creation.HighDateTime, creation.LowDateTime)
}
