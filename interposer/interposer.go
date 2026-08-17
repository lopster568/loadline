// Package interposer is a call-logging proxy for stdio MCP servers.
//
// It sits between an MCP client and a server: the client launches the
// interposer instead of the server, the interposer spawns the real server and
// relays stdin and stdout byte-for-byte in both directions, and every
// newline-delimited JSON-RPC frame is appended to a JSONL log.
//
// The relay path is the product. It must not reorder, rewrite, or drop bytes,
// and it must not add buffering that a server could observe as a timing
// change. Logging happens after the bytes have already been handed on.
package interposer

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Version is the interposer's semantic version. Per the loadline methodology,
// results are comparable only within one interposer version, so this constant
// is stamped into the header line of every log file.
const Version = "0.1.0"

// Options configures a single proxy session.
type Options struct {
	// LogPath is the JSONL frame log. Opened append-only, created 0600.
	LogPath string
	// FullResults logs complete response payloads instead of summaries.
	FullResults bool
	// ServerCmd is the argv of the real MCP server. Required.
	ServerCmd []string

	// The three stream ends. Zero values mean the process's own stdio.
	ClientIn  io.Reader
	ClientOut io.Writer
	ServerErr io.Writer
}

// Run proxies one client/server session and returns the server's exit code.
// A non-nil error means the session never started; the returned code is then
// the interposer's own failure code, not the server's.
func Run(o Options) (int, error) {
	if len(o.ServerCmd) == 0 {
		return 2, errors.New("no server command given")
	}
	if o.LogPath == "" {
		return 2, errors.New("--log is required")
	}
	if o.ClientIn == nil {
		o.ClientIn = os.Stdin
	}
	if o.ClientOut == nil {
		o.ClientOut = os.Stdout
	}
	if o.ServerErr == nil {
		o.ServerErr = os.Stderr
	}

	// 0600: frame logs carry full tool-call arguments, which routinely
	// include credentials and customer data.
	logFile, err := os.OpenFile(o.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return 2, err
	}
	defer logFile.Close()
	lg := &logger{w: logFile, full: o.FullResults}

	cmd := exec.Command(o.ServerCmd[0], o.ServerCmd[1:]...)
	cmd.Stderr = o.ServerErr
	serverIn, err := cmd.StdinPipe()
	if err != nil {
		return 2, err
	}
	serverOut, err := cmd.StdoutPipe()
	if err != nil {
		return 2, err
	}
	if err := cmd.Start(); err != nil {
		return 2, err
	}
	lg.header(o.ServerCmd, cmd.Process.Pid)

	// Forward interrupts so the server tears down the way it would have if
	// the client had launched it directly.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	go func() {
		for s := range sigs {
			_ = cmd.Process.Signal(s)
		}
	}()

	go func() {
		relay(o.ClientIn, serverIn, dirC2S, lg)
		// Client stdin closed: propagate EOF so the server shuts down.
		serverIn.Close()
	}()

	// Server output is drained to completion before reaping, so no frame is
	// lost to the exit. The client-to-server goroutine is deliberately not
	// waited on: a server that dies while the client still holds stdin open
	// would otherwise hang the interposer forever.
	relay(serverOut, o.ClientOut, dirS2C, lg)
	return exitCode(cmd.Wait()), nil
}

const (
	dirC2S = "c2s"
	dirS2C = "s2c"
)

// relay copies newline-delimited frames from src to dst without altering a
// byte, logging each one after it has been forwarded. A trailing chunk with no
// newline is forwarded and logged as its own frame.
func relay(src io.Reader, dst io.Writer, dir string, lg *logger) {
	br := bufio.NewReaderSize(src, 64*1024)
	for {
		// ReadBytes grows without bound, so multi-MB tool results and
		// partial reads both come through whole.
		frame, readErr := br.ReadBytes('\n')
		if len(frame) > 0 {
			_, writeErr := dst.Write(frame)
			lg.frame(dir, frame)
			if writeErr != nil {
				return
			}
		}
		if readErr != nil {
			return
		}
	}
}

// exitCode maps the result of cmd.Wait to a process exit code. A server killed
// by a signal has no exit status of its own and reports 1.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}
