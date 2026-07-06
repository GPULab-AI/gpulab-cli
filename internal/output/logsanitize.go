package output

import (
	"encoding/binary"
	"strings"
	"unicode/utf8"
)

// SanitizeLogs makes raw container log output safe to print and grep. It strips
// Docker's stream-multiplexing frame headers (which appear as binary garbage
// when logs come from a non-TTY container) and drops stray control bytes, while
// preserving newlines, tabs, and ANSI color escapes.
func SanitizeLogs(raw string) string {
	if raw == "" {
		return ""
	}
	data := []byte(raw)
	if demuxed, ok := demuxDockerStream(data); ok {
		data = demuxed
	}
	return stripControlBytes(data)
}

// demuxDockerStream strips Docker's 8-byte stream-multiplexing frame headers.
// Each frame is [stream(0|1|2)][0][0][0][uint32 big-endian length][payload].
// If the buffer does not begin with a plausible frame header it is returned
// unchanged (ok=false), so plain-text logs pass through untouched.
func demuxDockerStream(data []byte) ([]byte, bool) {
	if !looksLikeFrameHeader(data) {
		return data, false
	}
	out := make([]byte, 0, len(data))
	i := 0
	for i+8 <= len(data) {
		if data[i] > 2 || data[i+1] != 0 || data[i+2] != 0 || data[i+3] != 0 {
			// Header we can't trust — keep the remainder verbatim.
			out = append(out, data[i:]...)
			return out, true
		}
		n := int(binary.BigEndian.Uint32(data[i+4 : i+8]))
		i += 8
		if n < 0 || i+n > len(data) {
			out = append(out, data[i:]...)
			return out, true
		}
		out = append(out, data[i:i+n]...)
		i += n
	}
	if i < len(data) {
		out = append(out, data[i:]...)
	}
	return out, true
}

// looksLikeFrameHeader reports whether data starts with a Docker stream frame
// header. Plain text starts with a printable byte (>= 0x20) or newline, so it
// never matches; only the 0/1/2 stream markers with a zeroed high length do.
func looksLikeFrameHeader(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	if data[0] > 2 || data[1] != 0 || data[2] != 0 || data[3] != 0 {
		return false
	}
	// The two high bytes of the length are almost always zero for real frames;
	// requiring it rules out coincidental control-char prefixes in text.
	return data[4] == 0 && data[5] == 0
}

// stripControlBytes removes non-printable bytes that make logs unreadable and
// break grep, keeping newlines, tabs, carriage returns, and ANSI escapes.
func stripControlBytes(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for _, r := range string(data) {
		switch {
		case r == '\n' || r == '\t' || r == '\r' || r == 0x1b:
			b.WriteRune(r)
		case r == utf8.RuneError:
			// Drop invalid UTF-8 bytes.
		case r < 0x20 || r == 0x7f:
			// Drop other C0 control chars (incl. NUL and frame markers) and DEL.
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
