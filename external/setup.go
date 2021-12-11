package external

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/influxdata/toml"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/external/input"
	"github.com/influxdata/telegraf/models"
	"github.com/influxdata/telegraf/plugins/inputs"
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
		"input": &input.ExternalPlugin{Plugin: impl},
	}

	// Startup the server (the plugin) for us to connect to
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins:         plugins,
		Logger:          hclog.NewNullLogger(),
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}

func ConfigurePlugin(config string, v interface{}) error {
	// Strip the subtable
	parts := strings.SplitAfterN(config, "\n", 2)
	if len(parts) < 2 {
		return nil
	}
	table, err := toml.Parse([]byte(parts[1]))
	if err != nil {
		return err
	}
	return toml.UnmarshalTable(table, v)
}

// SetupReceiver provides the GRPC machinery for communicating with an external plugin
func SetupReceiver(cmd *exec.Cmd) *plugin.Client {
	// List all available plugin types
	plugins := map[string]plugin.Plugin{
		"input": &input.ExternalPlugin{},
	}

	// Startup the client (us) to call functions in the server (the plugin) and receive data
	return plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          plugins,
		Cmd:              cmd,
		Managed:          true,
		Logger:           hclog.NewNullLogger(),
		Stderr:           &logger{},
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})
}

// NewInputWrapper creates a new input plugin wrapper for an external plugin
func NewInputWrapper(name, root string) inputs.CreatorExternal {
	path := filepath.Join(root, "inputs", name)
	// Return an creator for a creator wrapping the configuration handling
	return func(config string) inputs.Creator {
		// Welcome to the real inputs.Creator function...
		return func() telegraf.Input {
			wrapper, err := input.NewWrapper(name, config, SetupReceiver(exec.Command(path)))
			if err != nil {
				panic(err)
			}
			return wrapper
		}
	}
}

// Discover checks the given directory and collects the paths to all executables in the category subdirectories
func Discover(root string) (map[string][]string, error) {
	register := make(map[string][]string)

	for _, category := range []string{"inputs", "outputs", "processors", "aggregators"} {
		categoryPath := filepath.Join(root, category)
		register[category] = make([]string, 0)

		// Check if the category directory
		fi, err := os.Stat(categoryPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("checking directory %q failed: %v", categoryPath, err)
		}
		if !fi.IsDir() {
			continue
		}

		// Walk the category directory and collect all excutables
		err = filepath.Walk(categoryPath, func(path string, info os.FileInfo, err error) error {
			wfi := info
			// Symlinks are not followed, but we want to only collect executables. Therefore, call stat,
			// resolving the link for us and gives the actual target properties
			if wfi.Mode()&os.ModeSymlink != 0 {
				var werr error
				wfi, werr = os.Stat(path)
				if werr != nil {
					log.Printf("W! Bad symbolic link %q", path)
					return nil
				}
			}
			// Check if the target is an executable file and add it
			if wfi.Mode()&fs.ModeType != 0 || wfi.Mode()&0111 == 0 {
				return nil
			}
			register[category] = append(register[category], wfi.Name())
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("path cannot be walked: %v", err)
		}
	}

	return register, nil
}
