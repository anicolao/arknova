package activation

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestListenRejectsActivationForAnotherProcess(t *testing.T) {
	t.Setenv("LISTEN_FDS", "1")
	t.Setenv("LISTEN_PID", "1")
	if os.Getpid() == 1 {
		t.Skip("test process unexpectedly has pid 1")
	}
	if _, _, err := Listen("127.0.0.1:0"); err == nil {
		t.Fatal("expected mismatched LISTEN_PID to be rejected")
	}
}

func TestListenCreatesConfiguredListenerWithoutActivation(t *testing.T) {
	t.Setenv("LISTEN_FDS", "")
	listener, activated, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if activated {
		t.Fatal("ordinary listener reported as systemd-activated")
	}
}

func TestListenUsesSystemdSocket(t *testing.T) {
	if os.Getenv("ARKNOVA_ACTIVATION_TEST_HELPER") == "1" {
		t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
		listener, activated, err := Listen("not-a-valid-address")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		fmt.Printf("activated=%t address=%s\n", activated, listener.Addr())
		return
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tcpListener := listener.(*net.TCPListener)
	file, err := tcpListener.File()
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	defer file.Close()
	defer listener.Close()

	command := exec.Command(os.Args[0], "-test.run=^TestListenUsesSystemdSocket$")
	command.ExtraFiles = []*os.File{file}
	command.Env = append(os.Environ(),
		"ARKNOVA_ACTIVATION_TEST_HELPER=1",
		"LISTEN_FDS=1",
		"LISTEN_FDNAMES=http",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("activation helper failed: %v\n%s", err, output)
	}
	want := "activated=true address=" + listener.Addr().String()
	if !strings.Contains(string(output), want) {
		t.Fatalf("activation helper output %q does not contain %q", output, want)
	}
}
