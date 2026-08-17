package mcpwire

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// StdioConfig describes a subprocess server launch. Methodology 1.2 requires
// the environment and argument vector to be recorded, so both are explicit
// here rather than inherited from the harness process.
type StdioConfig struct {
	Command string
	Args    []string
	Env     []string
	Dir     string
}

type stdioTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan []byte
	readErr chan error

	// done is closed by Close and unparks the stdout reader. Without it a
	// server that keeps writing after the harness has abandoned its wait fills
	// the 32-line buffer and parks the reader on the send forever, leaking a
	// goroutine and the subprocess pipe behind it for the rest of the sweep.
	done      chan struct{}
	closeOnce sync.Once
	// readerExited is closed when the stdout reader returns.
	readerExited chan struct{}

	mu     sync.Mutex
	stderr strings.Builder

	pending map[int64][]byte
	closed  bool
}

const stderrCaptureLimit = 8 << 10

// DialStdio launches the server subprocess and starts the reader loop.
func DialStdio(ctx context.Context, cfg StdioConfig) (Transport, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = cfg.Env
	cmd.Dir = cfg.Dir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch %s: %w", cfg.Command, err)
	}

	t := &stdioTransport{
		cmd:          cmd,
		stdin:        stdin,
		lines:        make(chan []byte, 32),
		readErr:      make(chan error, 1),
		done:         make(chan struct{}),
		readerExited: make(chan struct{}),
		pending:      map[int64][]byte{},
	}

	go func() {
		defer close(t.readerExited)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64<<10), 64<<20)
		for sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			select {
			case t.lines <- line:
			case <-t.done:
				return
			}
		}
		err := sc.Err()
		if err == nil {
			err = io.EOF
		}
		select {
		case t.readErr <- err:
		case <-t.done:
		}
	}()

	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 8<<10), 1<<20)
		for sc.Scan() {
			t.mu.Lock()
			if t.stderr.Len() < stderrCaptureLimit {
				t.stderr.Write(sc.Bytes())
				t.stderr.WriteByte('\n')
			}
			t.mu.Unlock()
		}
	}()

	return t, nil
}

func (t *stdioTransport) SetProtocolVersion(string) {}

func (t *stdioTransport) Diagnostics() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(t.stderr.String())
}

func (t *stdioTransport) Notify(ctx context.Context, payload []byte) error {
	return t.write(payload)
}

func (t *stdioTransport) Call(ctx context.Context, id int64, payload []byte) ([]byte, error) {
	if buffered, ok := t.pending[id]; ok {
		delete(t.pending, id)
		return buffered, nil
	}
	if err := t.write(payload); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-t.readErr:
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("server closed stdout before responding: %s", t.Diagnostics())
			}
			return nil, err
		case line := <-t.lines:
			trimmed := strings.TrimSpace(string(line))
			if trimmed == "" || trimmed[0] != '{' {
				// stdio servers are not supposed to write non-protocol text to
				// stdout, but several do; those lines are skipped, not fatal.
				continue
			}
			var probe struct {
				ID json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal(line, &probe); err != nil {
				continue
			}
			if len(probe.ID) == 0 || string(probe.ID) == "null" {
				continue
			}
			var got int64
			if err := json.Unmarshal(probe.ID, &got); err != nil {
				// The harness only ever issues integer ids, so a string id
				// belongs to a frame that is not ours.
				continue
			}
			if got != id {
				t.pending[got] = line
				continue
			}
			return line, nil
		}
	}
}

func (t *stdioTransport) write(payload []byte) error {
	if t.closed {
		return errors.New("transport closed")
	}
	if _, err := t.stdin.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write to server stdin: %w", err)
	}
	return nil
}

func (t *stdioTransport) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	// Unpark the reader before killing the process: cmd.Wait closes the stdout
	// pipe, and a reader parked on a channel send would never come back round
	// to notice.
	t.closeOnce.Do(func() { close(t.done) })
	_ = t.stdin.Close()
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	_ = t.cmd.Wait()
	return nil
}
