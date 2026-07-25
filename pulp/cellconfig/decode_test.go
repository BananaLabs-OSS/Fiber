package cellconfig

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestDecodeUsesJSONFieldNames(t *testing.T) {
	payload, err := msgpack.Marshal(map[string]any{
		"cpu_budget":       8.5,
		"memory_budget":    32.0,
		"service_endpoint": "https://example.test",
	})
	if err != nil {
		t.Fatal(err)
	}

	var config struct {
		CPU      float64 `json:"cpu_budget"`
		Memory   float64 `json:"memory_budget"`
		Endpoint string  `json:"service_endpoint"`
	}
	if err := Decode(payload, &config); err != nil {
		t.Fatal(err)
	}
	if config.CPU != 8.5 || config.Memory != 32 || config.Endpoint != "https://example.test" {
		t.Fatalf("decoded config = %+v", config)
	}
}

func TestDecodeRejectsMalformedMessagePack(t *testing.T) {
	var config struct{}
	if err := Decode([]byte{0xc1}, &config); err == nil {
		t.Fatal("Decode() accepted malformed MessagePack")
	}
}
