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
//
// Stripping is byte-transparent everywhere except complete OSC spans
// (ESC] ... (BEL|ESC\)). ESC keypresses are real input and must reach
// tea: a lone ESC is delivered at the end of the read that carries it,
// so it is never swallowed and never merged with the next key. The one
// consequence is that an OSC reply split exactly after its ESC byte
// (payload arriving in a later read) is not stripped — terminals write
// replies as a single chunk, so the ESC keypress wins that trade.
//
// Output is bounded: the expansion a pending ESC can produce never
// writes past the caller's buffer. Overflow is kept in the spill and
// returned on the next Read, so keystrokes chunked in any way must not
// crash the reader (a lone ESC followed by a key used to panic with an
// index-out-of-range in the writer).
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
	// spill holds filtered output that did not fit the previous Read
	// buffer. Stripping can expand input (a lone ESC followed by a key
	// yields two bytes for two bytes of input), so output must never be
	// written past the caller's buffer: overflow lands here and is
	// returned on the next Read.
	spill []byte
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

// Read reads from the wrapped reader, strips OSC spans, and writes the
// filtered bytes into p. Output is bounded: the expansion a pending ESC
// can produce never writes past the caller's buffer — overflow is kept
// in the spill and returned on the next Read, so a Read must never panic
// regardless of how keystrokes are chunked.
func (f *oscStrippingReader) Read(p []byte) (int, error) {
	// Drain overflow from the previous Read first.
	w := 0
	for w < len(p) && len(f.spill) > 0 {
		p[w] = f.spill[0]
		f.spill = f.spill[1:]
		w++
	}
	if w == len(p) {
		return w, nil
	}

	// Read a fresh chunk (a full p would only re-drain the spill).
	buf := make([]byte, len(p))
	n, err := f.r.Read(buf)
	if n == 0 {
		return w, err
	}

	emit := func(b byte) {
		if w < len(p) {
			p[w] = b
			w++
			return
		}
		f.spill = append(f.spill, b)
	}

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
			// The pending byte was a real ESC keypress (or the start of
			// a non-OSC sequence): pass it through before the byte that
			// resolved it, so ESC keypresses survive and still read as
			// ESC when a key follows in a later chunk.
			emit(0x1b)
			if b == 0x1b {
				f.escPending = true
				continue
			}
			emit(b)
		default:
			if b == 0x1b {
				f.escPending = true
			} else {
				emit(b)
			}
		}
	}

	// A read that ends on a lone ESC must deliver it: an ESC keypress is
	// usually its own read, and holding it back swallows the key. The
	// only cost is that an OSC span split exactly after ESC (payload in
	// the next read) is no longer stripped — replies arrive as a single
	// chunk in practice, so the real ESC keypress wins the trade.
	if f.escPending {
		emit(0x1b)
	}
	f.escPending = false

	if w > 0 && err == io.EOF {
		// Deliver the filtered bytes; the EOF surfaces on the next read.
		return w, nil
	}
	return w, err
}
