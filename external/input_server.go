package external

import (
  "context"

  "github.com/hashicorp/go-plugin"

  "github.com/influxdata/telegraf"
  "github.com/influxdata/telegraf/external/protocol"
)

type ExternalInputServer struct {
  Plugin telegraf.ExternalInput
  broker *plugin.GRPCBroker
  protocol.UnimplementedInputServer
}

func (s *ExternalInputServer) Description(context.Context, *protocol.Empty) (*protocol.DescriptionResponse, error) {
  descr := s.Plugin.Description()
  return &protocol.DescriptionResponse{Description: descr}, nil
}

func (s *ExternalInputServer) SampleConfig(context.Context, *protocol.Empty) (*protocol.SampleConfigResponse, error) {
  config := s.Plugin.SampleConfig()
  return &protocol.SampleConfigResponse{Config: config}, nil
}

func (s *ExternalInputServer) Configure(_ context.Context, req *protocol.ConfigureRequest) (*protocol.Error, error) {
  err := s.Plugin.Configure(req.Config)
  return ToErrorMessage(err), nil
}

func (s *ExternalInputServer) Init(context.Context, *protocol.Empty) (*protocol.Error, error) {
  err := s.Plugin.Init()
  return ToErrorMessage(err), nil
}

func (s *ExternalInputServer) Gather(context.Context, *protocol.Empty) (*protocol.GatherResponse, error) {
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
