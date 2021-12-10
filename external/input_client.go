 package external

import (
  "context"
  "fmt"

  "github.com/hashicorp/go-plugin"

  "github.com/influxdata/telegraf"
  "github.com/influxdata/telegraf/external/protocol"
)

type ExternalInputClient struct {
 	broker *plugin.GRPCBroker
	client protocol.InputClient
}

func (c *ExternalInputClient) Description() string {
  resp, err := c.client.Description(context.Background(), &protocol.Empty{})
	if err != nil {
    panic(fmt.Errorf("gRPC call failed: %v", err))
	}

	return resp.GetDescription()
}

func (c *ExternalInputClient) SampleConfig() string {
  resp, err := c.client.SampleConfig(context.Background(), &protocol.Empty{})
	if err != nil {
		panic(fmt.Errorf("gRPC call failed: %v", err))
	}

	return resp.GetConfig()
}

func (c *ExternalInputClient) Configure(config string) error {
  resp, err := c.client.Configure(context.Background(), &protocol.ConfigureRequest{Config: config})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %v", err)
	}

	return FromErrorMessage(resp)
}

func (c * ExternalInputClient) Init() error {
  resp, err := c.client.Init(context.Background(), &protocol.Empty{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %v", err)
	}

	return FromErrorMessage(resp)
}

func (c *ExternalInputClient) Gather() ([]telegraf.Metric, error) {
  resp, err := c.client.Gather(context.Background(), &protocol.Empty{})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %v", err)
	}

  if err := FromErrorMessage(resp.GetError()); err != nil {
    return nil, err
  }

	return FromMetricsMessage(resp.GetMetric()), nil
}
