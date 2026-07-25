//go:build wasip1

package workflow

import "github.com/BananaLabs-OSS/Fiber/pulp"

func defaultPulpCall(target, function string, payload []byte) ([]byte, error) {
	return pulp.Call(target, function, payload)
}
