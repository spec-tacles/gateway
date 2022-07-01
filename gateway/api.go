package gateway

import (
	"io"
	"net/http"

	"github.com/spec-tacles/go/types"
)

// DefaultVersion represents the default Gateway version
const DefaultVersion uint = 10

// Endpoints used for the Gateway
const (
	EndpointGateway    = "/gateway"
	EndpointGatewayBot = EndpointGateway + "/bot"
)

// REST represents a simple REST API handler
type REST interface {
	DoJSON(string, string, io.Reader, interface{}) error
}

// FetchGatewayBot fetches bot Gateway information
func FetchGatewayBot(rest REST) (*types.GatewayBot, error) {
	g := new(types.GatewayBot)
	return g, rest.DoJSON(http.MethodGet, EndpointGatewayBot, nil, g)
}
