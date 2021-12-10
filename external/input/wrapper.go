package input

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-plugin"

	"github.com/influxdata/telegraf"
)

type Wrapper struct {
	Log telegraf.Logger `toml:"-"`

	name   string
	config string
	plugin telegraf.ExternalInput
}

// External plugin support functions
func NewWrapper(name, config string, client *plugin.Client) (*Wrapper, error) {
	w := Wrapper{
		name:   name,
		config: config,
	}

	// Connect
	clientProto, err := client.Client()
	if err != nil {
		return nil, fmt.Errorf("connecting to external plugin %q failed: %v", w.name, err)
	}

	// Request the plugin
	raw, err := clientProto.Dispense("input")
	if err != nil {
		return nil, fmt.Errorf("cannot dispense input plugin: %v", err)
	}

	// Store the plugin for later calls
	plugin, ok := raw.(telegraf.ExternalInput)
	if !ok {
		return nil, errors.New("external plugin is not an input plugin")
	}
	w.plugin = plugin
	return &w, nil
}

func (w *Wrapper) Description() string {
	return w.plugin.Description()
}

func (w *Wrapper) SampleConfig() string {
	return w.plugin.SampleConfig()
}

func (w *Wrapper) Init() error {
	if err := w.plugin.Configure(w.config); err != nil {
		return fmt.Errorf("configuration failed: %v", err)
	}
	return w.plugin.Init()
}

func (w *Wrapper) Gather(acc telegraf.Accumulator) error {
	w.Log.Debugf("gather plugin: %v", w.plugin)

	metrics, err := w.plugin.Gather()
	if err != nil {
		return err
	}
	w.Log.Debugf("received %d metrics", len(metrics))
	for _, m := range metrics {
		acc.AddMetric(m)
	}

	return nil
}
