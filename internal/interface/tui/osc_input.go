package tui

import (
	"io"
	"os"
)

// oscStrippingReader filters terminal replies out of the input stream
// before bubbletea's parser sees them. tea has no OSC grammar:
// detectOneMsg does not recognize ESC] at all, so an OSC reply like
// ESC]11;rgb:1919/1aa/1b1bESC\ is decoded as plain runes and typed
// into the focused composer. The reply is only meaningful to the
// terminal drivers that requested it (they read /dev/tty, not stdin),
// so dropping the spans here costs nothing and keeps the composer
// clean.
//
// The filter MUST keep presenting itself as the real tty: tea only
// enters raw mode when the input is a term.File, and without raw mode
// the terminal line-buffers every keypress (j/k/Tab never arrive until
// a newline). So the wrapper forwards Fd/Write/Close to the underlying
// file while its Read path does the stripping.
type oscStrippingReader struct {
	r io.Reader
	// file is the real tty when wrapping one (os.Stdin at startup).
	file *os.File
	// escPending means a lone ESC byte was consumed waiting for ']'.
	// inOSC means we are inside ESC] ... (BEL|ESC\) and dropping; a
	// terminator ESC may straddle read boundaries (oscESCSeen).
	escPending bool
	inOSC      bool
	oscESCSeen bool
}

// newOSCStrippingReader wraps a real tty, keeping its identity intact.
func newOSCStrippingReader(f *os.File) *oscStrippingReader {
	return &oscStrippingReader{r: f, file: f}
}

// Fd implements term.File so tea treats the stripped input as the tty
// and enables raw mode (the raw-mode decision runs on Fd, the bytes
// still flow through Read).
func (f *oscStrippingReader) Fd() uintptr {
	if f.file != nil {
		return f.file.Fd()
	}
	return ^uintptr(0)
}

// Write forwards to the underlying tty (required by term.File).
func (f *oscStrippingReader) Write(p []byte) (int, error) {
	if f.file != nil {
		return f.file.Write(p)
	}
	return 0, io.ErrClosedPipe
}

// Close closes the underlying tty unless it is stdin.
func (f *oscStrippingReader) Close() error {
	if f.file != nil && f.file != os.Stdin {
		return f.file.Close()
	}
	return nil
}

// Name implements cancelreader.File, which requires io.ReadWriteCloser,
// Fd and Name. Without it cancelreader falls back to a reader that cannot
// cancel blocked reads; with it the app keeps the epoll fast path it had
// with the raw os.Stdin.
func (f *oscStrippingReader) Name() string {
	if f.file != nil {
		return f.file.Name()
	}
	return ""
}

// Read reads from the wrapped reader and strips OSC spans in place.
func (f *oscStrippingReader) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if n == 0 {
		return 0, err
	}
	buf := p[:n]
	w := 0
	for i := 0; i < n; i++ {
		b := buf[i]
		switch {
		case f.inOSC:
			if f.oscESCSeen {
				f.oscESCSeen = false
				if b == '\\' {
					f.inOSC = false
				}
				continue
			}
			if b == '\a' {
				f.inOSC = false
				continue
			}
			if b == 0x1b {
				if i+1 < n && buf[i+1] == '\\' {
					i++
					f.inOSC = false
				} else {
					f.oscESCSeen = true
				}
				continue
			}
			// Payload bytes are dropped.
		case f.escPending:
			f.escPending = false
			if b == ']' {
				f.inOSC = true
				continue
			}
			buf[w] = 0x1b
			w++
			if b == 0x1b {
				f.escPending = true
				continue
			}
			buf[w] = b
			w++
		default:
			if b == 0x1b {
				f.escPending = true
			} else {
				buf[w] = b
				w++
			}
		}
	}
	if w > 0 && err == io.EOF {
		// Deliver the filtered bytes; the EOF surfaces on the next read.
		return w, nil
	}
	return w, err
}
