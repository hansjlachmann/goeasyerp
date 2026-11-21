package appbundle

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"
)

// The listening socket across a switch: a server that starts another program (a newer bundle,
// or the standard program on its way to one) hands its socket over instead of closing it. The
// port stays open, so the web server in front never finds it closed (502): connections wait in
// the socket's queue while the programs change, and the new program answers them at once with
// the maintenance notice "starting" until it is ready (ServeStarting, Take).

// EnvListenFD names the handed-over socket's descriptor for the program exec starts
const EnvListenFD = "GOEASYERP_LISTEN_FD"

// handedOver keeps the handed-over descriptor open (an os.File closes it when collected)
var handedOver *os.File

// HandOver keeps ln's socket open for the program the next exec starts: a copy of its
// descriptor that survives the exec, named in the environment. ln itself may be closed
// afterwards (the server's shutdown); the copy holds the port.
func HandOver(ln net.Listener) error {
	fl, ok := ln.(interface{ File() (*os.File, error) })
	if !ok {
		return fmt.Errorf("the listener %T cannot be handed over", ln)
	}
	f, err := fl.File()
	if err != nil {
		return err
	}
	// through SyscallConn, not Fd (fdOf)
	raw, err := f.SyscallConn()
	if err != nil {
		_ = f.Close()
		return err
	}
	var errno syscall.Errno
	err = raw.Control(func(d uintptr) {
		// the copy is close-on-exec, as every descriptor Go opens: the exec must keep it
		_, _, errno = syscall.Syscall(syscall.SYS_FCNTL, d, syscall.F_SETFD, 0)
	})
	if err == nil && errno != 0 {
		err = errno
	}
	if err != nil {
		_ = f.Close()
		return err
	}
	if handedOver != nil {
		_ = handedOver.Close()
	}
	handedOver = f
	return os.Setenv(EnvListenFD, strconv.FormatUint(uint64(fdOf(f)), 10))
}

// fdOf is a file's descriptor without Fd, which would put the socket into blocking mode (also
// for the listener still serving: the copies share the socket's flags); ^0 when it has none
func fdOf(f *os.File) uintptr {
	fd := ^uintptr(0)
	raw, err := f.SyscallConn()
	if err == nil {
		_ = raw.Control(func(d uintptr) { fd = d })
	}
	return fd
}

// Inherited is the socket the program that started this one handed over (nil without one); the
// environment variable is removed, so a program this one starts does not take it as well
func Inherited() (net.Listener, error) {
	value := os.Getenv(EnvListenFD)
	if value == "" {
		return nil, nil
	}
	_ = os.Unsetenv(EnvListenFD)
	fd, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("%s=%q: %w", EnvListenFD, value, err)
	}
	// handed over by this same process (maintenance before the starting notice): its own file,
	// so that closing it here leaves no stale file that would close a reused descriptor later
	f := handedOver
	handedOver = nil
	if f == nil || fdOf(f) != uintptr(fd) {
		f = os.NewFile(uintptr(fd), "listener")
	}
	defer func() { _ = f.Close() }()
	syscall.CloseOnExec(fd)
	return net.FileListener(f)
}

// Starting answers on a handed-over socket while the program starts: every API request with the
// maintenance notice "starting" (the page checks again and opens by itself), /health with 200
type Starting struct {
	ln  net.Listener
	srv *http.Server
}

// ServeStarting serves the starting notice on the inherited socket; nil when there is none (a
// start that is not a switch: the server listens when it is ready, as before)
func ServeStarting() *Starting {
	ln, err := Inherited()
	if err != nil {
		log.Printf("app builds: the handed-over socket: %v", err)
		return nil
	}
	if ln == nil {
		return nil
	}
	s := &Starting{ln: ln, srv: &http.Server{
		Handler:           maintenanceHandler(&MaintenanceStatus{starting: true}),
		ReadHeaderTimeout: 10 * time.Second,
	}}
	go func() {
		err := s.srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("app builds: starting notice: %v", err)
		}
	}()
	return s
}

// Take stops the starting notice and gives its socket to the server (nil on a nil Starting: the
// server listens itself). The socket is copied first: stopping the notice closes its listener,
// the copy keeps the port and its waiting connections.
func (s *Starting) Take() (net.Listener, error) {
	if s == nil {
		return nil, nil
	}
	fl, ok := s.ln.(interface{ File() (*os.File, error) })
	if !ok {
		return nil, fmt.Errorf("the listener %T cannot be taken over", s.ln)
	}
	f, err := fl.File()
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
	return net.FileListener(f)
}

// HandOver hands the starting notice's socket to the program the next exec starts (nothing on a
// nil Starting)
func (s *Starting) HandOver() error {
	ln, err := s.Take()
	if err != nil || ln == nil {
		return err
	}
	err = HandOver(ln)
	_ = ln.Close()
	return err
}

// Listen is the server's socket: the starting notice's when the program took one over, else a
// new one on addr
func Listen(s *Starting, addr string) (net.Listener, error) {
	ln, err := s.Take()
	if err != nil {
		log.Printf("app builds: taking over the socket: %v (listening anew)", err)
	}
	if ln != nil {
		return ln, nil
	}
	return net.Listen("tcp", addr)
}
