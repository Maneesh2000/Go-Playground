// Package executor abstracts how a user's Go program is compiled and run:
// locally as a subprocess (dev) or inside a Kubernetes Job (prod). Program
// output is streamed back to the caller through an emit callback so the
// frontend terminal can render it live.
package executor

import "context"

// Stream names passed to EmitFunc.
const (
	StreamStdout = "stdout"
	StreamStderr = "stderr"
)

// EmitFunc receives a chunk of program output as it is produced.
// Implementations may be called from multiple goroutines concurrently.
type EmitFunc func(stream, data string)

// ExecRequest describes one run of an arbitrary program.
type ExecRequest struct {
	Code        string
	TimeLimitMS int // hard wall-clock limit for the program
}

// ExecResult is the final outcome of a run. Status uses the run status
// vocabulary (success, compile_error, runtime_error, ...). The JSON tags
// match the runner-image result-line contract.
type ExecResult struct {
	Status    string `json:"status"`
	ExitCode  int    `json:"exit_code"`
	RuntimeMS int    `json:"runtime_ms"`
	ErrorMsg  string `json:"error"`
}

// Executor runs a program to completion, streaming output via emit.
type Executor interface {
	Execute(ctx context.Context, req ExecRequest, emit EmitFunc) (ExecResult, error)
}
