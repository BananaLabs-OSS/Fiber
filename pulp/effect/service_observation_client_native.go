//go:build !wasip1

package effect

func hostServiceObservationExecute(reqPtr, reqLen, respPtrOut, respLenOut uint32) uint32 {
	return 99
}

func callHostServiceObservationWire(request []byte) ([]byte, uint32) {
	return nil, hostServiceObservationExecute(0, uint32(len(request)), 0, 0)
}

var serviceObservationExecuteWire = callHostServiceObservationWire
