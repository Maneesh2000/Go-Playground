package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/amura/codearena/internal/models"
)

const (
	compileTimeout = 30 * time.Second
	chunkSize      = 4 * 1024
)

// LocalExecutor compiles and runs the user's program as a subprocess on the
// worker host. This is the dev-mode default; it offers no sandboxing.
type LocalExecutor struct{}

// NewLocalExecutor returns a subprocess-based executor.
func NewLocalExecutor() *LocalExecutor { return &LocalExecutor{} }

// Execute writes the code to a temp dir, builds it with the Go toolchain and
// runs the binary, streaming its combined output through emit. The process
// group is killed hard when the time limit expires.
func (e *LocalExecutor) Execute(ctx context.Context, req ExecRequest, emit EmitFunc) (ExecResult, error) {
	dir, err := os.MkdirTemp("", "codearena-run-*")
	if err != nil {
		return ExecResult{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(req.Code), 0o600); err != nil {
		return ExecResult{}, fmt.Errorf("write source: %w", err)
	}

	// --- compile ---
	progPath := filepath.Join(dir, "prog")
	buildCtx, cancelBuild := context.WithTimeout(ctx, compileTimeout)
	defer cancelBuild()

	build := exec.CommandContext(buildCtx, "go", "build", "-o", progPath, "main.go")
	build.Dir = dir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	var buildOut bytes.Buffer
	build.Stdout = &buildOut
	build.Stderr = &buildOut

	if err := build.Run(); err != nil {
		if ctx.Err() != nil {
			return ExecResult{}, ctx.Err()
		}
		msg := buildOut.String()
		if errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
			msg = "compilation timed out after " + compileTimeout.String() + "\n"
		}
		// Stream the compiler output as stderr chunks, line by line.
		for _, line := range strings.SplitAfter(msg, "\n") {
			if line != "" {
				emit(StreamStderr, line)
			}
		}
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return ExecResult{
			Status:   models.StatusCompileError,
			ExitCode: exitCode,
			ErrorMsg: "compilation failed",
		}, nil
	}

	// --- run ---
	timeLimit := time.Duration(req.TimeLimitMS) * time.Millisecond
	if timeLimit <= 0 {
		timeLimit = 10 * time.Second
	}

	cmd := exec.Command(progPath)
	cmd.Dir = dir
	// Own process group so the whole tree (including children the program
	// spawns) can be killed at once.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ExecResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ExecResult{}, fmt.Errorf("stderr pipe: %w", err)
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return ExecResult{}, fmt.Errorf("start program: %w", err)
	}
	pgid := cmd.Process.Pid // Setpgid makes pgid == pid

	var timedOut atomic.Bool
	killTimer := time.AfterFunc(timeLimit, func() {
		timedOut.Store(true)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	})
	defer killTimer.Stop()

	// Kill the group if the worker is shutting down mid-run.
	runDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		case <-runDone:
		}
	}()

	// Stream both pipes concurrently. cmd.Wait must not be called until both
	// readers finish (it closes the pipes).
	var wg sync.WaitGroup
	wg.Add(2)
	go streamPipe(&wg, stdout, StreamStdout, emit)
	go streamPipe(&wg, stderr, StreamStderr, emit)
	wg.Wait()

	waitErr := cmd.Wait()
	close(runDone)
	runtimeMS := int(time.Since(start).Milliseconds())

	if ctx.Err() != nil {
		return ExecResult{}, ctx.Err()
	}

	if timedOut.Load() {
		return ExecResult{
			Status:    models.StatusTimeLimitExceeded,
			ExitCode:  exitCodeOf(cmd, waitErr),
			RuntimeMS: runtimeMS,
			ErrorMsg:  fmt.Sprintf("execution exceeded %s and was killed", formatLimit(timeLimit)),
		}, nil
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return ExecResult{}, fmt.Errorf("wait for program: %w", waitErr)
		}
		code := exitErr.ExitCode()
		return ExecResult{
			Status:    models.StatusRuntimeError,
			ExitCode:  code,
			RuntimeMS: runtimeMS,
			ErrorMsg:  fmt.Sprintf("process exited with code %d", code),
		}, nil
	}

	return ExecResult{
		Status:    models.StatusSuccess,
		ExitCode:  0,
		RuntimeMS: runtimeMS,
	}, nil
}

// streamPipe forwards a pipe to emit in bounded chunks (<=4KB) so even
// programs printing huge single lines stream without unbounded buffering.
func streamPipe(wg *sync.WaitGroup, r io.Reader, stream string, emit EmitFunc) {
	defer wg.Done()
	buf := make([]byte, chunkSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			emit(stream, string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func exitCodeOf(cmd *exec.Cmd, waitErr error) int {
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	if waitErr != nil {
		return -1
	}
	return 0
}

// formatLimit renders "10s" for whole seconds and "1.5s" otherwise, matching
// the human-facing error message contract.
func formatLimit(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return d.String()
}
