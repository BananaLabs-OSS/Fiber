package pulp

// Cell-side client for the discovery.mdns capability (Pulp-ext-mdns): browse the
// LAN for peers and announce this instance. The cell must declare "discovery.mdns".

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vmihailenco/msgpack/v5"
)

// MDNS groups the host-import wrappers for the discovery.mdns capability.
var MDNS = mdnsAPI{}

type mdnsAPI struct{}

// MDNSEntry is one discovered peer.
type MDNSEntry struct {
	Name string `msgpack:"name"`
	Addr string `msgpack:"addr"`
}

//go:wasmimport pulp mdns_browse
func hostMDNSBrowse(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32

//go:wasmimport pulp mdns_announce
func hostMDNSAnnounce(reqPtr, reqLen uint32) uint32

// Browse returns peers advertising service (default "_projx._tcp") seen within
// timeoutMs (default 2500).
func (mdnsAPI) Browse(service string, timeoutMs uint32) ([]MDNSEntry, error) {
	req, err := msgpack.Marshal(struct {
		Service   string `msgpack:"service"`
		TimeoutMs uint32 `msgpack:"timeout_ms"`
	}{Service: service, TimeoutMs: timeoutMs})
	if err != nil {
		return nil, err
	}
	var respPtr, respLen uint32
	code := hostMDNSBrowse(
		uint32(uintptr(unsafe.Pointer(&req[0]))), uint32(len(req)),
		uint32(uintptr(unsafe.Pointer(&respPtr))), uint32(uintptr(unsafe.Pointer(&respLen))),
	)
	runtime.KeepAlive(req)
	if code == 99 {
		return nil, ErrCapabilityUnavailable
	}
	if code != 0 {
		return nil, fmt.Errorf("mdns_browse host code %d", code)
	}
	if respLen == 0 {
		return nil, nil
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(respPtr))), respLen)
	buf := make([]byte, respLen)
	copy(buf, src)
	releaseHostAlloc(respPtr, respLen)
	var out []MDNSEntry
	if err := msgpack.Unmarshal(buf, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Announce advertises this instance on the LAN so peers can Browse it.
func (mdnsAPI) Announce(instance, service string, port uint32) error {
	req, err := msgpack.Marshal(struct {
		Instance string `msgpack:"instance"`
		Service  string `msgpack:"service"`
		Port     uint32 `msgpack:"port"`
	}{Instance: instance, Service: service, Port: port})
	if err != nil {
		return err
	}
	code := hostMDNSAnnounce(uint32(uintptr(unsafe.Pointer(&req[0]))), uint32(len(req)))
	runtime.KeepAlive(req)
	if code == 99 {
		return ErrCapabilityUnavailable
	}
	if code != 0 {
		return fmt.Errorf("mdns_announce host code %d", code)
	}
	return nil
}
