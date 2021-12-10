package external

import (
  "context"

	"github.com/hashicorp/go-plugin"
  "google.golang.org/grpc"

  "github.com/influxdata/telegraf"
  "github.com/influxdata/telegraf/external/protocol"
)

type ExternalInputPlugin struct {
  plugin.NetRPCUnsupportedPlugin
  Plugin telegraf.ExternalInput
}

func (p *ExternalInputPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	protocol.RegisterInputServer(s, &ExternalInputServer{
		Plugin: p.Plugin,
		broker: broker,
	})
	return nil
}

func (p *ExternalInputPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &ExternalInputClient{
		client: protocol.NewInputClient(c),
		broker: broker,
	}, nil
}
