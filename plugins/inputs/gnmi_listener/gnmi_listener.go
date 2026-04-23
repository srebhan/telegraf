//go:generate ../../../tools/config_includer/generator
//go:generate ../../../tools/readme_config_includer/generator
package gnmilistener

import (
	_ "embed"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/influxdata/telegraf"
	common_gnmi "github.com/influxdata/telegraf/plugins/common/gnmi"
	common_tls "github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/gnmi_listener/nokia"
)

//go:embed sample.conf
var sampleConfig string

type impl interface {
	Start(listener net.Listener, opts ...grpc.ServerOption) error
	Stop()
}

type GNMIListener struct {
	Address  string          `toml:"address"`
	Protocol string          `toml:"protocol"`
	Log      telegraf.Logger `toml:"-"`
	common_gnmi.HandlerConfig
	common_tls.ServerConfig

	options []grpc.ServerOption
	handler *common_gnmi.Handler
	server  impl
	addr    string
}

func (*GNMIListener) SampleConfig() string {
	return sampleConfig
}

func (g *GNMIListener) Init() error {
	// Defaults
	if g.Address == "" {
		g.Address = "localhost:57400"
	}

	// Check user settings
	switch g.Protocol {
	case "":
		g.Protocol = "nokia"
	case "nokia":
		// Do nothing, those are valid
	default:
		return fmt.Errorf("invalid 'protocol' %q", g.Protocol)
	}

	// Fill the server options depending on the user settings
	if tlsCfg, err := g.ServerConfig.TLSConfig(); err != nil {
		return fmt.Errorf("creating TLS configuration failed: %w", err)
	} else if tlsCfg != nil {
		g.options = append(g.options, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	if g.Log.Level().Includes(telegraf.Trace) {
		g.options = append(g.options, grpc.InTapHandle(g.logCalls))
	}

	return nil
}

func (g *GNMIListener) Start(acc telegraf.Accumulator) error {
	// Setup the handler
	h, err := common_gnmi.New(acc, g.HandlerConfig, g.Log)
	if err != nil {
		return fmt.Errorf("creating GNMI message handler failed: %w", err)
	}
	h.DefaultName = "gnmi"
	g.handler = h

	// Setup the server
	// Create the protocol implementation
	switch g.Protocol {
	case "nokia":
		g.server = nokia.New(g.handler.Handle, g.Log)
	default:
		return fmt.Errorf("invalid 'protocol' %q", g.Protocol)
	}

	// Create a listener or wrap it for debugging
	listener, err := net.Listen("tcp", g.Address)
	if err != nil {
		return fmt.Errorf("listening on %q failed: %w", g.Address, err)
	}
	g.addr = listener.Addr().String()

	// Start the server
	return g.server.Start(listener, g.options...)
}

func (g *GNMIListener) Stop() {
	if g.server != nil {
		g.server.Stop()
	}
}

func (*GNMIListener) Gather(telegraf.Accumulator) error {
	return nil
}

func init() {
	inputs.Add("gnmi_listener", func() telegraf.Input {
		return &GNMIListener{}
	})
}
