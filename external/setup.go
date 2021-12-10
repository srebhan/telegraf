package external

import (
  "os/exec"

  "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

  "github.com/influxdata/telegraf"
  "github.com/influxdata/telegraf/models"
)

var handshake = plugin.HandshakeConfig{
  ProtocolVersion:  1,
  MagicCookieKey:   "TELEGRAF_PLUGIN",
  MagicCookieValue: "f2d7c59c-bd1a-4f38-8543-7d1d51b81d49",
}

// SetupInputPlugin can be used in the external plugin to setup GRPC machinery for an external input plugin
// This function has to be called in the plugin!
func SetupInputPlugin(name, alias string, impl telegraf.ExternalInput) {
  models.SetLoggerOnPlugin(impl, models.NewLogger("inputs", name, alias))

  // List all available plugin types
  plugins := map[string]plugin.Plugin{
    "input": &externalInputPlugin{Plugin: impl},
  }

  // Startup the server (the plugin) for us to connect to
  plugin.Serve(&plugin.ServeConfig{
    HandshakeConfig: handshake,
    Plugins:         plugins,
    Logger:          hclog.NewNullLogger(),
    GRPCServer:      plugin.DefaultGRPCServer,
  })
}

// SetupReceiver provides the GRPC machinery for communicating with an external plugin
func SetupReceiver(cmd *exec.Cmd) *plugin.Client {
  // List all available plugin types
  plugins := map[string]plugin.Plugin{
    "input": &externalInputPlugin{},
  }

  // Startup the client (us) to call functions in the server (the plugin) and receive data
  return plugin.NewClient(&plugin.ClientConfig{
    HandshakeConfig: handshake,
    Plugins:         plugins,
    Cmd:             cmd,
    Logger:					 hclog.NewNullLogger(),
    Stderr:          &logger{},
    AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
  })
}
