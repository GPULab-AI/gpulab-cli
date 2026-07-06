package output

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// frame builds a Docker stream-multiplexed frame: [stream][0 0 0][uint32 len][payload].
func frame(stream byte, payload []byte) []byte {
	hdr := make([]byte, 8)
	hdr[0] = stream
	binary.BigEndian.PutUint32(hdr[4:], uint32(len(payload)))
	return append(hdr, payload...)
}

func TestSanitizeLogs_PlainTextUnchanged(t *testing.T) {
	in := "current queue time: 0\nready\n"
	if got := SanitizeLogs(in); got != in {
		t.Fatalf("plain text changed: %q", got)
	}
}

func TestSanitizeLogs_DemuxesFrames(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(frame(1, []byte("hello\n")))
	buf.Write(frame(2, []byte("world\n")))
	got := SanitizeLogs(buf.String())
	if got != "hello\nworld\n" {
		t.Fatalf("demux failed: %q", got)
	}
}

func TestSanitizeLogs_LargeFrameOver64KiB(t *testing.T) {
	// A frame larger than 64 KiB must still be detected and de-framed
	// (regression: the old heuristic required the two high length bytes to be 0).
	big := bytes.Repeat([]byte("a"), 70000)
	var buf bytes.Buffer
	buf.Write(frame(1, big))
	buf.Write(frame(1, []byte("\ntail\n")))
	got := SanitizeLogs(buf.String())
	if !strings.HasSuffix(got, "tail\n") || len(got) != 70000+len("\ntail\n") {
		t.Fatalf("large frame not de-framed cleanly: len=%d suffix=%q", len(got), got[len(got)-6:])
	}
	if strings.ContainsRune(got, 0x01) {
		t.Fatalf("frame header bytes leaked into output")
	}
}

func TestSanitizeLogs_StripsControlBytes(t *testing.T) {
	got := SanitizeLogs("ok\x00\x01text\x07\n")
	if got != "oktext\n" {
		t.Fatalf("control bytes not stripped: %q", got)
	}
}
