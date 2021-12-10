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

func SetupInputPlugin(name, alias string, impl telegraf.ExternalInput) {
  models.SetLoggerOnPlugin(impl, models.NewLogger("inputs", name, alias))

  // List all available plugin types
  plugins := map[string]plugin.Plugin{
    "input": &ExternalInputPlugin{Plugin: impl},
  }

  plugin.Serve(&plugin.ServeConfig{
    HandshakeConfig: handshake,
    Plugins:         plugins,
    Logger:          hclog.NewNullLogger(),
    GRPCServer:      plugin.DefaultGRPCServer,
  })
}

func SetupReceiver(cmd string) *plugin.Client {
  // List all available plugin types
  plugins := map[string]plugin.Plugin{
    "input": &ExternalInputPlugin{},
  }

  // We're a host! Start by launching the plugin process.
  return plugin.NewClient(&plugin.ClientConfig{
    HandshakeConfig: handshake,
    Plugins:         plugins,
    Cmd:             exec.Command(cmd),
    Logger:					 hclog.NewNullLogger(),
    Stderr:          &Logger{},
    AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
  })
}
