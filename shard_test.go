package gateway_test

import (
	"testing"

	"github.com/spec-tacles/spectacles.go/gateway"
)

func TestShard(t *testing.T) {
	s := gateway.NewShard("MjE4ODQ0NDIwNjEzNzM0NDAx.DwitmA._wvtidIcvQvXIQeS6zT2xAgoErs", 0)
	s.Connect()
}
