// Module 13, example 2: running external commands with os/exec —
// capturing output, exit codes, pipes between commands, and timeouts.
//
// Run with: go run 02_exec_commands.go
//
// Uses only ubiquitous Unix commands (echo, ls, sort, head, sh, sleep) so it
// runs on macOS and Linux out of the box.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	// ----------------------- 1) capture stdout --------------------------
	// Arguments are a LIST — "echo" never sees a shell, so there is no
	// quoting hell and no injection risk. $HOME below is NOT expanded:
	out, err := exec.Command("echo", "hello from a child process, $HOME stays literal").Output()
	must(err)
	fmt.Printf("Output(): %s", out)

	// ---------------- 2) capture stdout AND stderr ----------------------
	// ls of a missing file writes to stderr and exits non-zero.
	// CombinedOutput interleaves both streams — great for diagnostics.
	out, err = exec.Command("ls", "/definitely/not/a/path").CombinedOutput()
	fmt.Printf("CombinedOutput(): err=%v, output=%s", err, out)

	// -------------------------- 3) exit codes ---------------------------
	// A non-zero exit becomes an *exec.ExitError. errors.As digs it out.
	err = exec.Command("sh", "-c", "exit 3").Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		fmt.Println("exit code was:", exitErr.ExitCode()) // 3
	}
	// Distinguish "ran and failed" (ExitError) from "couldn't even start"
	// (e.g. command not found) — different error type:
	err = exec.Command("no-such-binary-xyz").Run()
	fmt.Println("couldn't start:", err, "| is ExitError?", errors.As(err, &exitErr))

	// ----------------- 4) feed stdin, read stdout -----------------------
	// Equivalent of: printf 'cherry\napple\nbanana\n' | sort
	sortCmd := exec.Command("sort")
	sortCmd.Stdin = strings.NewReader("cherry\napple\nbanana\n") // any io.Reader
	out, err = sortCmd.Output()
	must(err)
	fmt.Printf("sorted via `sort`:\n%s", out)

	// ------------------- 5) pipe: cmd1 | cmd2 ---------------------------
	// Go equivalent of: echo -e "..." | head -2
	// Connect cmd1's StdoutPipe to cmd2's Stdin, start both, wait for both.
	cmd1 := exec.Command("echo", "line1\nline2\nline3\nline4")
	cmd2 := exec.Command("head", "-2")

	pipe, err := cmd1.StdoutPipe() // read end of cmd1's stdout
	must(err)
	cmd2.Stdin = pipe

	var result bytes.Buffer
	cmd2.Stdout = &result

	must(cmd2.Start()) // start the DOWNSTREAM first so it's ready to read
	must(cmd1.Start())
	must(cmd1.Wait()) // wait for the producer...
	must(cmd2.Wait()) // ...then the consumer
	fmt.Printf("echo | head -2:\n%s", result.String())

	// ------------- 6) stream output line-by-line while running ----------
	// Don't wait for the process to finish — scan its output live.
	// (Think: tailing the output of a long build.)
	long := exec.Command("sh", "-c", `for i in 1 2 3; do echo "step $i"; done`)
	stdout, err := long.StdoutPipe()
	must(err)
	must(long.Start())
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		fmt.Println("live:", sc.Text())
	}
	must(long.Wait()) // Wait AFTER draining the pipe, or you can deadlock

	// -------------------- 7) timeout via context ------------------------
	// CommandContext kills the process when the ctx expires. Here `sleep 5`
	// gets 200ms to live.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = exec.CommandContext(ctx, "sleep", "5").Run()
	fmt.Printf("sleep 5 with 200ms budget: err=%v after %v\n",
		err, time.Since(start).Round(time.Millisecond))

	// ------------------ 8) environment & working dir --------------------
	pwd := exec.Command("sh", "-c", `echo "cwd=$(pwd) GREETING=$GREETING"`)
	pwd.Dir = os.TempDir()                           // run in another directory
	pwd.Env = append(os.Environ(), "GREETING=hello") // inherit + add
	out, err = pwd.Output()
	must(err)
	fmt.Printf("custom dir+env: %s", out)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
