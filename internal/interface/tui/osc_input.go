package tui

import "io"

// oscStrippingReader filters terminal replies out of the input stream
// before bubbletea's parser sees them. tea has no OSC grammar:
// detectOneMsg does not recognize ESC] at all, so an OSC reply like
// ESC]11;rgb:1919/1aa/1b1bESC\ is decoded as plain runes and typed
// into the focused composer. The reply is only meaningful to the
// terminal drivers that requested it (they read /dev/tty, not stdin),
// so dropping the spans here costs nothing and keeps the composer
// clean.
type oscStrippingReader struct {
	r io.Reader
	// escPending means a lone ESC byte was consumed waiting for ']'.
	// inOSC means we are inside ESC] ... (BEL|ESC\) and dropping; a
	// terminator ESC may straddle read boundaries (oscESCSeen).
	escPending bool
	inOSC      bool
	oscESCSeen bool
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
