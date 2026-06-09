package agent

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Term is one interactive shell session backed by a PTY. Output read from the
// PTY is delivered via the onData callback; onExit fires once when the shell
// ends. All methods are safe for concurrent use.
type Term struct {
	root string

	mu   sync.Mutex
	ptmx *os.File
	cmd  *exec.Cmd
}

// NewTerm returns a Term whose shell starts in dir root.
func NewTerm(root string) *Term { return &Term{root: root} }

// Start launches a login shell in a PTY sized cols x rows. onData receives raw
// PTY output; onExit is called with the shell's exit code when it terminates.
func (t *Term) Start(cols, rows uint16, onData func([]byte), onExit func(int)) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ptmx != nil {
		return nil // already started; ignore duplicate start
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell, "-l")
	cmd.Dir = t.root
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	_ = pty.Setsize(ptmx, &pty.Winsize{Cols: cols, Rows: rows})
	t.ptmx = ptmx
	t.cmd = cmd

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				onData(chunk)
			}
			if err != nil {
				break
			}
		}
		code := 0
		if err := cmd.Wait(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				code = 1
			}
		}
		onExit(code)
	}()
	return nil
}

// Write forwards keystrokes/stdin to the shell.
func (t *Term) Write(data []byte) {
	t.mu.Lock()
	ptmx := t.ptmx
	t.mu.Unlock()
	if ptmx != nil {
		_, _ = ptmx.Write(data)
	}
}

// Resize updates the PTY window size (so full-screen TUIs render correctly).
func (t *Term) Resize(cols, rows uint16) {
	t.mu.Lock()
	ptmx := t.ptmx
	t.mu.Unlock()
	if ptmx != nil {
		_ = pty.Setsize(ptmx, &pty.Winsize{Cols: cols, Rows: rows})
	}
}

// Close terminates the shell and releases the PTY.
func (t *Term) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ptmx != nil {
		_ = t.ptmx.Close()
		t.ptmx = nil
	}
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
}
