package edge

import (
	"encoding/binary"
	"errors"
	"testing"
)

func mcVarInt(value int) []byte {
	v := uint32(value)
	var out []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}

func mcHandshake(protocol int, host string, port uint16, state int) []byte {
	body := append(mcVarInt(0), mcVarInt(protocol)...)
	body = append(body, mcVarInt(len(host))...)
	body = append(body, host...)
	p := make([]byte, 2)
	binary.BigEndian.PutUint16(p, port)
	body = append(body, p...)
	body = append(body, mcVarInt(state)...)
	return append(mcVarInt(len(body)), body...)
}

func TestMinecraftHandshakeFragmented(t *testing.T) {
	wire := mcHandshake(774, "play.example.test", 25565, 2)
	m, err := NewMinecraftHandshake(1024)
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Open(Connection{ID: "tcp:1", Transport: "tcp"})
	var observations []Observation
	for _, chunk := range [][]byte{wire[:1], wire[1:4], wire[4:]} {
		got, err := m.Push(chunk)
		if err != nil {
			t.Fatal(err)
		}
		observations = append(observations, got...)
	}
	if len(observations) != 1 || observations[0].Kind != "minecraft.handshake" {
		t.Fatalf("observations=%#v", observations)
	}
	fields := observations[0].Fields
	if fields["protocol_version"] != "774" || fields["server_address"] != "play.example.test" || fields["server_port"] != "25565" || fields["next_state"] != "2" {
		t.Fatalf("fields=%#v", fields)
	}
	if string(m.BufferedWire()) != string(wire) || !m.Complete() {
		t.Fatal("wire was not retained exactly")
	}
}

func TestMinecraftHandshakeRejectsOversizeBeforePayload(t *testing.T) {
	m, _ := NewMinecraftHandshake(32)
	_, err := m.Push(mcVarInt(33))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestMinecraftHandshakeRejectsMalformedVarInt(t *testing.T) {
	m, _ := NewMinecraftHandshake(1024)
	_, err := m.Push([]byte{0x80, 0x80, 0x80, 0x80, 0x80})
	if !errors.Is(err, ErrMalformedVarInt) {
		t.Fatalf("err=%v", err)
	}
}

func TestMinecraftHandshakeRejectsTrailingFields(t *testing.T) {
	wire := append(mcHandshake(774, "play.example.test", 25565, 2), 0)
	// Increase the declared frame size so the trailing byte is inside the frame.
	wire[0]++
	m, _ := NewMinecraftHandshake(1024)
	_, err := m.Push(wire)
	if !errors.Is(err, ErrMalformedHandshake) {
		t.Fatalf("err=%v", err)
	}
}
