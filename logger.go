package envx

import (
	"fmt"
	"io"
	"os"
)

// Logger defines minimal logging used inside the package.
type Logger interface {
	Printf(format string, args ...any)
}

type writerLogger struct {
	w io.Writer
}

func (l writerLogger) Printf(format string, args ...any) {
	fmt.Fprintf(l.w, format, args...)
}

func newWriterLogger(w io.Writer) Logger {
	if w == nil {
		w = os.Stdout
	}
	return writerLogger{w: w}
}
