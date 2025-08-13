package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ensureSingleInstance terminates any previously running instance of this
// service and writes the current process PID to a file so subsequent runs can
// replace it.
func ensureSingleInstance() (func(), error) {
	pidFile := filepath.Join(os.TempDir(), "fs-mcp-go.pid")
	exePath, _ := os.Executable()
	execName := filepath.Base(exePath)

	if b, err := os.ReadFile(pidFile); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(b)), ":", 2)
		if len(parts) == 2 && parts[1] == execName {
			if old, err := strconv.Atoi(parts[0]); err == nil {
				if p, err := os.FindProcess(old); err == nil {
					_ = p.Kill()
				}
			}
		}
	}
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d:%s", os.Getpid(), execName)), 0o644); err != nil {
		return nil, err
	}
	return func() { os.Remove(pidFile) }, nil
}
