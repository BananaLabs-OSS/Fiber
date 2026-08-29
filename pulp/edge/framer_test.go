package edge

import (
	"encoding/binary"
	"errors"
	"testing"
)

func encoded(values ...string) []byte {
	var out []byte
	for _, value := range values {
		header := make([]byte, 4)
		binary.BigEndian.PutUint32(header, uint32(len(value)))
		out = append(out, header...)
		out = append(out, value...)
	}
	return out
}

func TestLengthPrefixFramerFragmentedAndCoalesced(t *testing.T) {
	f, _ := NewLengthPrefixFramer(64)
	wire := encoded("auth one-use-code", "message hello")
	var got [][]byte
	for _, part := range [][]byte{wire[:2], wire[2:7], wire[7:]} {
		frames, err := f.Push(part)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, frames...)
	}
	if len(got) != 2 || string(got[0]) != "auth one-use-code" || string(got[1]) != "message hello" {
		t.Fatalf("frames=%q", got)
	}
	if f.Buffered() != 0 {
		t.Fatalf("buffered=%d", f.Buffered())
	}
}

func TestLengthPrefixFramerRejectsOversizeBeforePayload(t *testing.T) {
	f, _ := NewLengthPrefixFramer(8)
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, 9)
	if _, err := f.Push(header); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestLengthPrefixFramerSplitsLargeCoalescedBatch(t *testing.T) {
	f, _ := NewLengthPrefixFramer(64)
	values := make([]string, 10000)
	for i := range values {
		values[i] = "message"
	}
	frames, err := f.Push(encoded(values...))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != len(values) || f.Buffered() != 0 {
		t.Fatalf("frames=%d buffered=%d", len(frames), f.Buffered())
	}
}
