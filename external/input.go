package external

import (
  "context"

	"github.com/hashicorp/go-plugin"
  "google.golang.org/grpc"

  "github.com/influxdata/telegraf"
  "github.com/influxdata/telegraf/external/protocol"
)

type externalInputPlugin struct {
  plugin.NetRPCUnsupportedPlugin
  Plugin telegraf.ExternalInput
}

func (p *externalInputPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	protocol.RegisterInputServer(s, &externalInputServer{
		Plugin: p.Plugin,
		broker: broker,
	})
	return nil
}

func (p *externalInputPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &externalInputClient{
		client: protocol.NewInputClient(c),
		broker: broker,
	}, nil
}
