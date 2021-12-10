package external

import (
  "context"
  "fmt"

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

// Server implementation
type externalInputServer struct {
  Plugin telegraf.ExternalInput
  broker *plugin.GRPCBroker
  protocol.UnimplementedInputServer
}

func (s *externalInputServer) Description(context.Context, *protocol.Empty) (*protocol.DescriptionResponse, error) {
  descr := s.Plugin.Description()
  return &protocol.DescriptionResponse{Description: descr}, nil
}

func (s *externalInputServer) SampleConfig(context.Context, *protocol.Empty) (*protocol.SampleConfigResponse, error) {
  config := s.Plugin.SampleConfig()
  return &protocol.SampleConfigResponse{Config: config}, nil
}

func (s *externalInputServer) Configure(_ context.Context, req *protocol.ConfigureRequest) (*protocol.Error, error) {
  err := s.Plugin.Configure(req.Config)
  return ToErrorMessage(err), nil
}

func (s *externalInputServer) Init(context.Context, *protocol.Empty) (*protocol.Error, error) {
  err := s.Plugin.Init()
  return ToErrorMessage(err), nil
}

func (s *externalInputServer) Gather(context.Context, *protocol.Empty) (*protocol.GatherResponse, error) {
  metrics, err := s.Plugin.Gather()
  if err != nil {
    return &protocol.GatherResponse{Error: ToErrorMessage(err)}, nil
  }

  msgmetrics, err := ToMetricsMessage(metrics)
  if err != nil  {
    return nil, err
  }

  return &protocol.GatherResponse{Metric: msgmetrics}, nil
}

// Client implementation
type externalInputClient struct {
 	broker *plugin.GRPCBroker
	client protocol.InputClient
}

func (c *externalInputClient) Description() string {
  resp, err := c.client.Description(context.Background(), &protocol.Empty{})
	if err != nil {
    panic(fmt.Errorf("gRPC call failed: %v", err))
	}

	return resp.GetDescription()
}

func (c *externalInputClient) SampleConfig() string {
  resp, err := c.client.SampleConfig(context.Background(), &protocol.Empty{})
	if err != nil {
		panic(fmt.Errorf("gRPC call failed: %v", err))
	}

	return resp.GetConfig()
}

func (c *externalInputClient) Configure(config string) error {
  resp, err := c.client.Configure(context.Background(), &protocol.ConfigureRequest{Config: config})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %v", err)
	}

	return FromErrorMessage(resp)
}

func (c * externalInputClient) Init() error {
  resp, err := c.client.Init(context.Background(), &protocol.Empty{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %v", err)
	}

	return FromErrorMessage(resp)
}

func (c *externalInputClient) Gather() ([]telegraf.Metric, error) {
  resp, err := c.client.Gather(context.Background(), &protocol.Empty{})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %v", err)
	}

  if err := FromErrorMessage(resp.GetError()); err != nil {
    return nil, err
  }

	return FromMetricsMessage(resp.GetMetric()), nil
}
