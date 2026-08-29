// Package edge defines transport-neutral protocol module contracts for
// programmable edge cells. Modules frame bytes and emit semantic observations;
// routing, identity authority, and application behavior stay outside modules.
package edge

type Mode string

const (
	ModeOpaque    Mode = "opaque"
	ModeClassify  Mode = "classify"
	ModeObserve   Mode = "observe"
	ModeTransform Mode = "transform"
	ModeTerminate Mode = "terminate"
)

type Capabilities struct {
	Mode      Mode   `json:"mode" msgpack:"mode"`
	Transport string `json:"transport" msgpack:"transport"`
	Framing   string `json:"framing" msgpack:"framing"`
	Identity  bool   `json:"identity" msgpack:"identity"`
	Messaging bool   `json:"messaging" msgpack:"messaging"`
	Handoff   string `json:"handoff" msgpack:"handoff"`
	MaxFrame  uint32 `json:"max_frame" msgpack:"max_frame"`
}

type Connection struct {
	ID             string `json:"id" msgpack:"id"`
	Transport      string `json:"transport" msgpack:"transport"`
	Listener       string `json:"listener" msgpack:"listener"`
	SourceEndpoint string `json:"source_endpoint" msgpack:"source_endpoint"`
}

type Observation struct {
	Kind    string            `json:"kind" msgpack:"kind"`
	Fields  map[string]string `json:"fields,omitempty" msgpack:"fields,omitempty"`
	Payload []byte            `json:"payload,omitempty" msgpack:"payload,omitempty"`
}

type Module interface {
	Name() string
	Capabilities() Capabilities
	Open(Connection) error
	Push([]byte) ([]Observation, error)
	Close(reason string)
}
