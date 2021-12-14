package input

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-plugin"
	"github.com/influxdata/toml"
	"google.golang.org/grpc"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/external/protocol"
)

type ExternalPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	Plugin telegraf.ExternalInput
}

func (p *ExternalPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	protocol.RegisterInputServer(s, &server{
		Plugin: p.Plugin,
		broker: broker,
	})
	return nil
}

func (p *ExternalPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &externalInputClient{
		client: protocol.NewInputClient(c),
		broker: broker,
	}, nil
}

// Server implementation
type server struct {
	Plugin telegraf.ExternalInput
	broker *plugin.GRPCBroker
	protocol.UnimplementedInputServer
}

func (s *server) Description(context.Context, *protocol.Empty) (*protocol.DescriptionResponse, error) {
	descr := s.Plugin.Description()
	return &protocol.DescriptionResponse{Description: descr}, nil
}

func (s *server) SampleConfig(context.Context, *protocol.Empty) (*protocol.SampleConfigResponse, error) {
	config := s.Plugin.SampleConfig()
	return &protocol.SampleConfigResponse{Config: config}, nil
}

func (s *server) Configure(_ context.Context, req *protocol.ConfigureRequest) (*protocol.Error, error) {
	config := req.Config

	// Strip the subtable
	parts := strings.SplitAfterN(config, "\n", 2)
	if len(parts) < 2 {
		return protocol.ToErrorMessage(nil), nil
	}
	table, err := toml.Parse([]byte(parts[1]))
	if err != nil {
		return protocol.ToErrorMessage(err), nil
	}
	err = toml.UnmarshalTable(table, s.Plugin)

	return protocol.ToErrorMessage(err), nil
}

func (s *server) Init(context.Context, *protocol.Empty) (*protocol.Error, error) {
	err := s.Plugin.Init()
	return protocol.ToErrorMessage(err), nil
}

func (s *server) Gather(context.Context, *protocol.Empty) (*protocol.GatherResponse, error) {
	metrics, err := s.Plugin.Gather()
	if err != nil {
		return &protocol.GatherResponse{Error: protocol.ToErrorMessage(err)}, nil
	}

	msgmetrics, err := protocol.ToMetricsMessage(metrics)
	if err != nil {
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

	return protocol.FromErrorMessage(resp)
}

func (c *externalInputClient) Init() error {
	resp, err := c.client.Init(context.Background(), &protocol.Empty{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %v", err)
	}

	return protocol.FromErrorMessage(resp)
}

func (c *externalInputClient) Gather() ([]telegraf.Metric, error) {
	resp, err := c.client.Gather(context.Background(), &protocol.Empty{})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %v", err)
	}

	if err := protocol.FromErrorMessage(resp.GetError()); err != nil {
		return nil, err
	}

	return protocol.FromMetricsMessage(resp.GetMetric()), nil
}
