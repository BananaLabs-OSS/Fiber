package edge

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrMalformedVarInt           = errors.New("malformed Minecraft VarInt")
	ErrMalformedHandshake        = errors.New("malformed Minecraft handshake")
	ErrUnsupportedHandshakeState = errors.New("unsupported Minecraft handshake state")
)

// MinecraftHandshake observes exactly one server-bound handshake packet. It
// deliberately does not proxy login, compression, or encryption; callers can
// replay BufferedWire to a selected backend and transition to an opaque native
// bridge after the observation is emitted.
type MinecraftHandshake struct {
	maxFrame uint32
	buffer   []byte
	done     bool
}

func NewMinecraftHandshake(maxFrame uint32) (*MinecraftHandshake, error) {
	if maxFrame == 0 || maxFrame > 64*1024 {
		return nil, fmt.Errorf("Minecraft handshake max frame must be between 1 and 65536 bytes")
	}
	return &MinecraftHandshake{maxFrame: maxFrame}, nil
}

func (m *MinecraftHandshake) Name() string { return "minecraft-handshake-v1" }

func (m *MinecraftHandshake) Capabilities() Capabilities {
	return Capabilities{Mode: ModeClassify, Transport: "tcp", Framing: "minecraft-varint", Handoff: "gateway", MaxFrame: m.maxFrame}
}

func (m *MinecraftHandshake) Open(Connection) error {
	m.buffer = nil
	m.done = false
	return nil
}

func (m *MinecraftHandshake) Push(chunk []byte) ([]Observation, error) {
	if m.done {
		return nil, nil
	}
	if uint64(len(m.buffer))+uint64(len(chunk)) > uint64(m.maxFrame)+5 {
		m.buffer = nil
		return nil, ErrFrameTooLarge
	}
	m.buffer = append(m.buffer, chunk...)
	frameSize, headerSize, complete, err := minecraftVarInt(m.buffer)
	if err != nil {
		return nil, err
	}
	if !complete {
		return nil, nil
	}
	if frameSize < 0 || uint32(frameSize) > m.maxFrame {
		return nil, ErrFrameTooLarge
	}
	needed := headerSize + frameSize
	if len(m.buffer) < needed {
		return nil, nil
	}
	fields, err := parseMinecraftHandshake(m.buffer[headerSize:needed])
	if err != nil {
		return nil, err
	}
	m.done = true
	return []Observation{{Kind: "minecraft.handshake", Fields: fields}}, nil
}

func (m *MinecraftHandshake) Close(string) { m.buffer = nil }

// BufferedWire returns the exact bytes consumed while classifying. The caller
// must replay these bytes unchanged before starting its opaque bridge.
func (m *MinecraftHandshake) BufferedWire() []byte { return append([]byte(nil), m.buffer...) }
func (m *MinecraftHandshake) Complete() bool       { return m.done }

func minecraftVarInt(raw []byte) (value int, width int, complete bool, err error) {
	var result uint32
	for i := 0; i < len(raw) && i < 5; i++ {
		b := raw[i]
		result |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			if i == 4 && b&0xf0 != 0 {
				return 0, 0, false, ErrMalformedVarInt
			}
			return int(int32(result)), i + 1, true, nil
		}
	}
	if len(raw) >= 5 {
		return 0, 0, false, ErrMalformedVarInt
	}
	return 0, 0, false, nil
}

func takeMinecraftVarInt(raw []byte, offset *int) (int, error) {
	value, width, complete, err := minecraftVarInt(raw[*offset:])
	if err != nil || !complete {
		if err != nil {
			return 0, err
		}
		return 0, ErrMalformedHandshake
	}
	*offset += width
	return value, nil
}

func parseMinecraftHandshake(frame []byte) (map[string]string, error) {
	offset := 0
	packetID, err := takeMinecraftVarInt(frame, &offset)
	if err != nil || packetID != 0 {
		return nil, ErrMalformedHandshake
	}
	protocol, err := takeMinecraftVarInt(frame, &offset)
	if err != nil || protocol < 0 {
		return nil, ErrMalformedHandshake
	}
	hostSize, err := takeMinecraftVarInt(frame, &offset)
	if err != nil || hostSize < 1 || hostSize > 255 || hostSize > len(frame)-offset {
		return nil, ErrMalformedHandshake
	}
	host := string(frame[offset : offset+hostSize])
	offset += hostSize
	if len(frame)-offset < 2 {
		return nil, ErrMalformedHandshake
	}
	port := binary.BigEndian.Uint16(frame[offset : offset+2])
	offset += 2
	nextState, err := takeMinecraftVarInt(frame, &offset)
	if err != nil || offset != len(frame) {
		return nil, ErrMalformedHandshake
	}
	if nextState != 1 && nextState != 2 {
		return nil, ErrUnsupportedHandshakeState
	}
	return map[string]string{
		"protocol_version": strconv.Itoa(protocol),
		"server_address":   host,
		"server_port":      strconv.Itoa(int(port)),
		"next_state":       strconv.Itoa(nextState),
	}, nil
}
