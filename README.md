# pty

A small Go package for starting commands attached to a pseudo terminal.

It supports Unix PTYs on Linux, macOS, DragonFly BSD, FreeBSD, NetBSD, and
OpenBSD, plus Windows ConPTY. Unsupported platforms return
`errors.ErrUnsupported`.

## Install

```sh
go get github.com/phuslu/pty
```

## Example

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/phuslu/pty"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.Command("/bin/sh", "-c", "printf 'hello from pty\\n'")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/d", "/c", "echo hello from pty")
	}

	ptmx, err := pty.StartWithSize(ctx, cmd, &pty.Winsize{Rows: 30, Cols: 100})
	if err != nil {
		panic(err)
	}
	defer ptmx.Close()

	if _, err := io.Copy(os.Stdout, ptmx); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
```

## API

- `Start(ctx, cmd)` starts `cmd` with stdin, stdout, and stderr connected to a PTY.
- `StartWithSize(ctx, cmd, size)` starts `cmd` with an initial terminal size.
- `SetSize(pty, size)` resizes a running PTY.
- `IsTerminal(fd)` reports whether a file descriptor, such as `os.Stdout.Fd()`, is a terminal.
