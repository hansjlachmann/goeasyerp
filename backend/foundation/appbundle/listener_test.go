package appbundle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// helperEnv makes the test binary the program of TestHandOverAcrossExec
const helperEnv = "APPBUNDLE_TEST_HELPER"

func init() {
	switch os.Getenv(helperEnv) {
	case "first":
		helperFirst()
	case "second":
		helperSecond()
	}
}

// helperFirst listens, tells the test its address, hands the socket over and closes its own
// listener — as a server switching does — and execs the second program after a pause, in
// which the test's requests wait in the socket's queue
func helperFirst() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	fmt.Println(ln.Addr().String())
	err = HandOver(ln)
	if err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	_ = ln.Close()
	time.Sleep(300 * time.Millisecond)
	_ = os.Setenv(helperEnv, "second")
	err = syscall.Exec(os.Args[0], os.Args, os.Environ())
	fmt.Println("error", err)
	os.Exit(1)
}

// helperSecond answers on the socket it was handed: the starting notice first, then the
// "server" (the answer "ready") once it took the socket
func helperSecond() {
	s := ServeStarting()
	if s == nil {
		os.Exit(2)
	}
	time.Sleep(500 * time.Millisecond)
	ln, err := Listen(s, "127.0.0.1:0")
	if err != nil {
		os.Exit(3)
	}
	_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ready")
	}))
	os.Exit(0)
}

// A server that switches programs keeps its port: every request meanwhile is answered — by the
// starting notice, then by the new program — never refused
func TestHandOverAcrossExec(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), helperEnv+"=first")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || strings.HasPrefix(line, "error") {
		t.Fatalf("helper: %q %v", line, err)
	}
	url := "http://" + strings.TrimSpace(line) + "/api/session"
	client := &http.Client{Timeout: 5 * time.Second}
	sawStarting := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("a request during the switch failed: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK && string(body) == "ready" {
			if !sawStarting {
				t.Error("the starting notice never answered")
			}
			return
		}
		var a maintenanceAnswer
		if resp.StatusCode != http.StatusServiceUnavailable || json.Unmarshal(body, &a) != nil || !a.Maintenance || !a.Starting {
			t.Fatalf("unexpected answer %d %s", resp.StatusCode, body)
		}
		sawStarting = true
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the new program never answered")
}

// The handed-over copy survives the exec (not close-on-exec), holds the port after the
// listener closed, and the next program takes it once
func TestHandOverAndInherited(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(EnvListenFD) })
	if err := HandOver(ln); err != nil {
		t.Fatal(err)
	}
	var flags uintptr
	raw, _ := handedOver.SyscallConn()
	_ = raw.Control(func(fd uintptr) {
		flags, _, _ = syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFD, 0)
	})
	if flags&syscall.FD_CLOEXEC != 0 {
		t.Error("the handed-over descriptor is close-on-exec")
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("the port closed with the listener: %v", err)
	}
	defer func() { _ = conn.Close() }()
	got, err := Inherited()
	if err != nil || got == nil {
		t.Fatalf("Inherited: %v %v", got, err)
	}
	defer func() { _ = got.Close() }()
	accepted, err := got.Accept()
	if err != nil {
		t.Fatalf("the waiting connection: %v", err)
	}
	_ = accepted.Close()
	if os.Getenv(EnvListenFD) != "" {
		t.Error("the variable stays for the next program")
	}
	if again, _ := Inherited(); again != nil {
		t.Error("taken twice")
	}
	if s := ServeStarting(); s != nil {
		t.Error("a starting notice without a handed-over socket")
	}
	if ln, err := Listen(nil, "127.0.0.1:0"); err != nil {
		t.Errorf("Listen without a starting notice: %v", err)
	} else {
		_ = ln.Close()
	}
}

// The launcher's sequence in one process: maintenance hands its socket over, the starting
// notice takes it, and the exec to the bundle hands it over again — the port open throughout
func TestHandOverTwiceInOneProcess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(EnvListenFD) })
	addr := ln.Addr().String()
	if err := HandOver(ln); err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	s := ServeStarting()
	if s == nil {
		t.Fatal("no starting notice")
	}
	resp, err := http.Get("http://" + addr + "/api/session")
	if err != nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("starting notice: %v %v", resp, err)
	}
	_ = resp.Body.Close()
	if err := s.HandOver(); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("the port closed between the programs: %v", err)
	}
	_ = conn.Close()
	again, err := Inherited()
	if err != nil || again == nil {
		t.Fatalf("the bundle's socket: %v %v", again, err)
	}
	_ = again.Close()
}
