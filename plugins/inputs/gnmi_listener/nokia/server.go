//go:generate protoc --proto_path=../gnmi_protos:. --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative nokia-dialout-telemetry.proto
package nokia

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/influxdata/telegraf"
	common_gnmi "github.com/influxdata/telegraf/plugins/common/gnmi"
)

// Make sure we implement the GRPC interface
var _ DialoutTelemetryServer = &server{}

type server struct {
	addr    string
	handler common_gnmi.HandlerFunc
	log     telegraf.Logger

	server *grpc.Server
	UnimplementedDialoutTelemetryServer
}

func New(handler common_gnmi.HandlerFunc, log telegraf.Logger) *server {
	return &server{
		handler: handler,
		log:     log,
	}
}

func (s *server) Start(addr string, opts ...grpc.ServerOption) error {
	// Create a listener for the server
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %q failed: %w", addr, err)
	}
	s.addr = listener.Addr().String()

	// Create the GRPC server and start it
	s.server = grpc.NewServer(opts...)
	RegisterDialoutTelemetryServer(s.server, s)
	go func() {
		if err := s.server.Serve(listener); err != nil {
			s.log.Errorf("GRPC server on %q got error: %w", addr, err)
		}
	}()

	return nil
}

func (s *server) Stop() {
	if s.server != nil {
		s.server.Stop()
	}
}

func (s *server) Address() string {
	return s.addr
}

// Publish implements the Nokia dial-out GRPC interface
func (s *server) Publish(srv grpc.BidiStreamingServer[gnmi.SubscribeResponse, gnmi.SubscribeRequest]) error {
	ctx := srv.Context()

	for ctx.Err() == nil {
		// Wait for data
		response, err := srv.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				return fmt.Errorf("aborted gNMI listener: %w", err)
			}
			break
		}

		// Determine the message source
		source := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			switch v := p.Addr.(type) {
			case *net.TCPAddr:
				source = v.IP.String()
			case *net.UDPAddr:
				source = v.IP.String()
			case *net.IPAddr:
				source = v.IP.String()
			default:
				source = p.Addr.String()
			}
		}

		// Call the handler
		if err := s.handler(source, response); err != nil {
			s.log.Errorf("Handling GNMI message failed: %v", err)
		}

		// Send an empty response
		srv.Send(&gnmi.SubscribeRequest{})
	}

	return nil
}
