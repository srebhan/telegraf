package external

import (
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-plugin"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/external"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type External struct {
	Name string          `toml:"name"`
	Path string          `toml:"path"`
	Log  telegraf.Logger `toml:"-"`

	client *plugin.Client
	plugin telegraf.ExternalInput
}

const sampleConfig = `
  ## Name of the plugin
  # name = "test"

  ## Path for executing the plugin
  path = "/path/to/your/plugin"

  ## Specify a duration allowing time-unit suffixes ('ns','ms', 's', 'm', etc.)
	# timeout = "100ms"
`

// Description will appear directly above the plugin definition in the config file
func (e *External) Description() string {
	return `Plugin for executing external plugins`
}

// SampleConfig will populate the sample configuration portion of the plugin's configuration
func (e *External) SampleConfig() string {
	cfg := e.plugin.SampleConfig()
	return sampleConfig + "\n" + cfg
}

func (e *External) Init() error {
	e.client = external.SetupReceiver(exec.Command(e.Path))

	return nil
}

func (e *External) Start(_ telegraf.Accumulator) error {
	e.Log.Debug("start called...")

	e.Log.Debugf("client: %v", e.client)

	// Connect
	clientProto, err := e.client.Client()
	if err != nil {
		return fmt.Errorf("connecting to external plugin %q failed: %v", e.Name, err)
	}
	e.Log.Debugf("proto: %v", clientProto)

	// Request the plugin
	raw, err := clientProto.Dispense("input")
	if err != nil {
		return fmt.Errorf("cannot dispense %q plugin: %v", "input", err)
	}
	e.Log.Debugf("raw plugin: %v (%T)", raw, raw)

	// Store the plugin for later calls
	plugin, ok := raw.(telegraf.ExternalInput)
	if !ok {
		return fmt.Errorf("external plugin is not an %q plugin", "input")
	}
	e.plugin = plugin
	e.Log.Debugf("started plugin: %v", e.plugin)
	description := e.plugin.Description()
	e.Log.Debug(description)

	// Setup the plugin
	if err := e.plugin.Configure(""); err != nil {
		return fmt.Errorf("configure failed: %v", err)
	}

	if err := e.plugin.Init(); err != nil {
		return fmt.Errorf("initialization failed: %v", err)
	}

	return nil
}

func (e *External) Stop() {
	e.Log.Debug("stop called...")
	e.client.Kill()
}

func (e *External) Gather(acc telegraf.Accumulator) error {
	e.Log.Debugf("gather plugin: %v", e.plugin)

	metrics, err := e.plugin.Gather()
	if err != nil {
		return err
	}
	e.Log.Debugf("received %d metrics", len(metrics))
	for _, m := range metrics {
		acc.AddMetric(m)
	}

	return nil
}

// Register the plugin
func init() {
	inputs.Add("external", func() telegraf.Input {
		return &External{Name: "test"}
	})
}
