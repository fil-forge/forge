package contentclaims

import (
	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/server"
	"github.com/fil-forge/ucantone/ucan"
)

func NewUCANServer(id ucan.Issuer, service types.Publisher, options ...server.HTTPOption) (*server.HTTPServer, error) {
	ucanServer := server.NewHTTP(id, options...)
	routes := NewUCANService(service)
	for _, route := range routes {
		ucanServer.Handle(route.Command, route.Handler)
	}
	return ucanServer, nil
}
