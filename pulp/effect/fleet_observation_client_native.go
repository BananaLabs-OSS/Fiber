//go:build !wasip1

package effect

// Native builds have no Pulp host. Tests may replace the package-local wire
// seam, while production native calls always fail closed with code 99.
func hostFleetObservationExecute(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 {
	return 99
}

func callHostFleetObservationWire(request []byte) ([]byte, uint32) {
	return nil, hostFleetObservationExecute(0, uint32(len(request)), 0, 0)
}

var fleetObservationExecuteWire = callHostFleetObservationWire
