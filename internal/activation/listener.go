package activation

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

const systemdListenFD = 3

func Listen(address string) (net.Listener, bool, error) {
	count := os.Getenv("LISTEN_FDS")
	if count == "" {
		listener, err := net.Listen("tcp", address)
		return listener, false, err
	}
	if os.Getenv("LISTEN_PID") != strconv.Itoa(os.Getpid()) {
		return nil, false, fmt.Errorf("LISTEN_PID does not identify this process")
	}
	if count != "1" {
		return nil, false, fmt.Errorf("expected exactly one systemd listener, got %q", count)
	}
	if names := os.Getenv("LISTEN_FDNAMES"); names != "" && names != "http" {
		return nil, false, fmt.Errorf("expected systemd listener named http, got %q", names)
	}

	file := os.NewFile(systemdListenFD, "systemd-http-listener")
	if file == nil {
		return nil, false, fmt.Errorf("systemd listener fd %d is unavailable", systemdListenFD)
	}
	listener, err := net.FileListener(file)
	_ = file.Close()
	if err != nil {
		return nil, false, fmt.Errorf("open systemd listener: %w", err)
	}
	for _, name := range []string{"LISTEN_PID", "LISTEN_FDS", "LISTEN_FDNAMES"} {
		_ = os.Unsetenv(name)
	}
	return listener, true, nil
}
