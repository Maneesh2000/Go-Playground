// Module 13, example 1: precise file opening (os.OpenFile), permissions,
// path manipulation, tree walking, temp files, and environment variables.
//
// Run with: go run 01_files_paths_env.go
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	// Sandbox: everything happens in a temp dir that we remove at the end.
	// os.MkdirTemp picks a unique name under the system temp directory —
	// safe even if many copies of this program run at once.
	root, err := os.MkdirTemp("", "module13-*")
	must(err)
	defer os.RemoveAll(root)
	fmt.Println("working in:", root)

	// ----------------------- os.OpenFile flags --------------------------
	logPath := filepath.Join(root, "app.log") // ALWAYS Join, never "+"

	// Append-mode logging: create if missing, write-only, append every write.
	// 0o644 = rw-r--r-- (owner read/write, everyone else read).
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	must(err)
	fmt.Fprintln(logFile, "first run")
	logFile.Close()

	// Open again — O_APPEND means we extend rather than overwrite.
	logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	must(err)
	fmt.Fprintln(logFile, "second run")
	logFile.Close()

	data, _ := os.ReadFile(logPath)
	fmt.Printf("app.log after two opens:\n%s", data)

	// O_EXCL: "create ONLY if it doesn't exist" — atomic, so it doubles as
	// a poor man's lockfile between processes.
	_, err = os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	fmt.Println("O_EXCL on existing file fails as expected:", err != nil)

	// Inspect metadata with Stat; Mode().Perm() shows the permission bits.
	fi, err := os.Stat(logPath)
	must(err)
	fmt.Printf("stat: name=%s size=%dB mode=%v\n", fi.Name(), fi.Size(), fi.Mode().Perm())

	// Tighten permissions: owner-only read/write (e.g. a credentials file).
	must(os.Chmod(logPath, 0o600))
	fi, _ = os.Stat(logPath)
	fmt.Println("after chmod 600:", fi.Mode().Perm())

	// -------------------------- path/filepath ---------------------------
	fmt.Println("--- filepath toolkit ---")
	p := filepath.Join(root, "src", "cmd", "main.go")
	fmt.Println("Join :", p)
	fmt.Println("Dir  :", filepath.Dir(p))
	fmt.Println("Base :", filepath.Base(p))
	fmt.Println("Ext  :", filepath.Ext(p))
	rel, _ := filepath.Rel(root, p)
	fmt.Println("Rel  :", rel)

	// ------------------------- walking a tree ---------------------------
	// Build a small tree to walk: MkdirAll creates parents as needed
	// (like mkdir -p). Note the 0o755 mode for directories (need +x to enter).
	for _, d := range []string{"src/cmd", "src/util", ".git/objects"} {
		must(os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	for _, f := range []string{"src/cmd/main.go", "src/util/strings.go", "README.md", ".git/objects/abc"} {
		must(os.WriteFile(filepath.Join(root, f), []byte("package demo\n"), 0o644))
	}

	fmt.Println("--- WalkDir (skipping .git) ---")
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // propagate I/O errors (permission denied etc.)
		}
		// fs.SkipDir tells WalkDir not to descend into this directory.
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		rel, _ := filepath.Rel(root, path)
		kind := "file"
		if d.IsDir() {
			kind = "dir "
		}
		fmt.Printf("  %s %s\n", kind, rel)
		return nil
	})
	must(err)

	// -------------------------- temp files ------------------------------
	// CreateTemp gives a unique name AND an open handle — no races with
	// other processes picking the same name.
	tmp, err := os.CreateTemp(root, "upload-*.partial")
	must(err)
	fmt.Fprintln(tmp, "partial data")
	tmp.Close()

	// The atomic-replace idiom: write the new content to a temp file, then
	// Rename over the destination. Readers see either the OLD complete file
	// or the NEW complete file — never a half-written one.
	final := filepath.Join(root, "config.yaml")
	must(os.Rename(tmp.Name(), final))
	fmt.Println("atomic replace via rename → ", filepath.Base(final))

	// --------------------- environment variables ------------------------
	fmt.Println("--- environment ---")
	// Getenv returns "" for unset vars — ambiguous if "" is a legal value.
	fmt.Println("HOME =", os.Getenv("HOME"))

	// LookupEnv disambiguates "unset" from "set to empty".
	if v, ok := os.LookupEnv("MODULE13_MODE"); ok {
		fmt.Println("MODULE13_MODE is set to:", v)
	} else {
		fmt.Println("MODULE13_MODE is not set (try: MODULE13_MODE=dev go run 01_files_paths_env.go)")
	}

	// Setenv affects THIS process and its future children — you can never
	// change your parent shell's environment.
	os.Setenv("MODULE13_STAMP", "set-from-go")
	fmt.Println("MODULE13_STAMP =", os.Getenv("MODULE13_STAMP"))
	fmt.Printf("total env vars visible: %d\n", len(os.Environ()))
}

// must keeps the example focused: real code returns errors, demos may bail.
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
