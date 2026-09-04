//go:build windows

package pty

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestComposeCommandLine(t *testing.T) {
	args := []string{`C:\Program Files\liner.exe`, `two words`, `quote"mark`}
	got := composeCommandLine(args)
	for _, arg := range args {
		if !strings.Contains(got, syscall.EscapeArg(arg)) {
			t.Fatalf("command line %q does not contain escaped arg %q", got, syscall.EscapeArg(arg))
		}
	}
}

func TestComposeCommandLineEdgeCases(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: nil, want: ""},
		{args: []string{""}, want: `""`},
		{args: []string{"a", "b c", `d"e`, `f\`, ""}, want: `a "b c" d\"e f\ ""`},
		{args: []string{`C:\Program Files\tool.exe`, "--x=hello world"}, want: `"C:\Program Files\tool.exe" "--x=hello world"`},
		{args: []string{`a\ b`}, want: `"a\ b"`},
		{args: []string{`a\ b"c`}, want: `"a\ b\"c"`},
		{args: []string{`\\server\share\file.txt`, `arg\\`}, want: `\\server\share\file.txt arg\\`},
	}
	for _, tt := range tests {
		if got := composeCommandLine(tt.args); got != tt.want {
			t.Errorf("composeCommandLine(%#v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestLookExtensions(t *testing.T) {
	t.Setenv("PATHEXT", ".EXE;.CMD")
	dir := t.TempDir()
	otherDir := t.TempDir()
	for _, name := range []string{"tool.EXE", "tool.cmd"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	absolute := filepath.Join(dir, "tool.cmd")
	tests := []struct {
		path string
		dir  string
		want string
	}{
		{path: "tool", dir: dir, want: "./tool.EXE"},
		{path: "tool.exe", dir: dir, want: "./tool.exe"},
		{path: "tool.cmd", dir: dir, want: "./tool.cmd"},
		{path: absolute, dir: otherDir, want: absolute},
	}
	for _, tt := range tests {
		got, err := lookExtensions(tt.path, tt.dir)
		if err != nil {
			t.Errorf("lookExtensions(%q, %q): %v", tt.path, tt.dir, err)
			continue
		}
		if got != tt.want {
			t.Errorf("lookExtensions(%q, %q) = %q, want %q", tt.path, tt.dir, got, tt.want)
		}
	}
}

func TestCreateEnvBlock(t *testing.T) {
	block, err := createEnvBlock([]string{"A=B", "C=D"})
	if err != nil {
		t.Fatalf("createEnvBlock: %v", err)
	}
	if len(block) == 0 || block[len(block)-1] != 0 {
		t.Fatalf("env block is not NUL terminated: %#v", block)
	}

	_, err = createEnvBlock([]string{"A=B\x00C"})
	if err == nil {
		t.Fatal("createEnvBlock accepted an environment variable containing NUL")
	}
}

func TestCreateEnvBlockSortsEnvironment(t *testing.T) {
	block, err := createEnvBlock([]string{"b=2", "A=1"})
	if err != nil {
		t.Fatalf("createEnvBlock: %v", err)
	}
	got := envBlockStrings(block)
	want := []string{"A=1", "b=2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("env block entries = %#v, want %#v", got, want)
	}
}

func TestCreateEnvBlockDeduplicatesEnvironment(t *testing.T) {
	block, err := createEnvBlock([]string{"A=1", "b=2", "a=3", "B=4"})
	if err != nil {
		t.Fatalf("createEnvBlock: %v", err)
	}
	got := envBlockStrings(block)
	want := []string{"a=3", "B=4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("env block entries = %#v, want %#v", got, want)
	}
}

func FuzzCreateEnvBlock(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		[]byte("A=B"),
		[]byte("A=1\na=2\nB=3\nb=4"),
		[]byte("=leading-equals"),
		[]byte("no-equals"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		env := strings.Split(string(data), "\n")
		block, err := createEnvBlock(env)
		if err != nil {
			for _, kv := range env {
				if strings.IndexByte(kv, 0) >= 0 {
					return
				}
			}
			t.Fatalf("createEnvBlock(%#v) error = %v", env, err)
		}
		if len(block) < 2 || block[len(block)-1] != 0 || block[len(block)-2] != 0 {
			t.Fatalf("env block %#v is not double NUL terminated", block)
		}

		entries := envBlockStrings(block)
		seen := make(map[string]bool, len(entries))
		var prev string
		for i, kv := range entries {
			key := strings.ToUpper(envKey(kv))
			if seen[key] {
				t.Fatalf("duplicate env key %q in block %#v", key, entries)
			}
			seen[key] = true
			if i > 0 && strings.ToUpper(envKey(prev)) > key {
				t.Fatalf("env block %#v is not sorted", entries)
			}
			prev = kv
		}
	})
}

func TestCoordPack(t *testing.T) {
	got := coord{X: 80, Y: 30}.pack()
	want := uintptr(uint32(80) | uint32(30)<<16)
	if got != want {
		t.Fatalf("coord.pack() = %#x, want %#x", got, want)
	}
}

func TestHRESULTError(t *testing.T) {
	if err := hresultError(0); err != nil {
		t.Fatalf("hresultError(0): %v", err)
	}

	err := hresultError(0x80070005)
	if !errors.Is(err, syscall.Errno(5)) {
		t.Fatalf("hresultError(0x80070005) = %v, want errno 5", err)
	}
}

func envBlockStrings(block []uint16) []string {
	var entries []string
	for start := 0; start < len(block); {
		end := start
		for end < len(block) && block[end] != 0 {
			end++
		}
		if end == start {
			break
		}
		entries = append(entries, syscall.UTF16ToString(block[start:end]))
		start = end + 1
	}
	return entries
}
