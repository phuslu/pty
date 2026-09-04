# pty

A small Go package for starting commands attached to a pseudo terminal.

It supports Unix PTYs on AIX, Linux, macOS, DragonFly BSD, FreeBSD, NetBSD,
OpenBSD, Solaris, and z/OS, plus Windows ConPTY. Unsupported platforms return
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

- `Start(ctx, cmd)` starts `cmd` with stdin, stdout, and stderr connected to a
  PTY. `cmd` becomes the leader of its own process group on Unix, and is
  assigned to a job object on Windows where possible, so cancellation of `ctx`
  terminates its descendants as well as `cmd` itself.
- `StartWithSize(ctx, cmd, size)` starts `cmd` with an initial terminal size.
  A nil size, or zero rows/columns, falls back to the default 80x30 size.
- `Open()` returns the pty master and slave ends without starting a command.
  On Unix both ends are real files; on Windows the returned slave only owns
  the internal ConPTY pipe handles (`Read`/`Write` return
  `errors.ErrUnsupported`).
- `GetSize(pty)` returns a running PTY's current terminal size. On Windows,
  where ConPTY has no size query API, it returns the most recent size applied
  through this package.
- `SetSize(pty, size)` resizes a running PTY. Both `Rows` and `Cols` must be
  non-zero.
- `IsTerminal(fd)` reports whether a file descriptor, such as `os.Stdout.Fd()`,
  is a terminal.
- `EnableVirtualTerminal(stdin, stdout, stderr)` enables ConHost VT input and
  output processing on the current process's standard handles. It is a no-op
  on Unix and unsupported platforms.

On Windows, `Pty.Fd()` returns the ConPTY `HANDLE` rather than an OS file
descriptor; it is only meaningful when passed back to this package, for
example to `SetSize`.
