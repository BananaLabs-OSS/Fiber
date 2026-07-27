//go:build !wasip1

package effect

func hostCapacityObservationExecute(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 {
	return 99
}

func callHostCapacityObservationWire(request []byte) ([]byte, uint32) {
	return nil, hostCapacityObservationExecute(0, uint32(len(request)), 0, 0)
}

var capacityObservationExecuteWire = callHostCapacityObservationWire
