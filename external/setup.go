package external

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/external/input"
	"github.com/influxdata/telegraf/models"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// ChecksumFilename is the name expected for checksum files created with the sha256sum tool
const ChecksumFilename = "checksums"

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

// SetupReceiver provides the GRPC machinery for communicating with an external plugin
func SetupReceiver(cmd *exec.Cmd, checksum string) (*plugin.Client, error) {
	var securecfg *plugin.SecureConfig

	// List all available plugin types
	plugins := map[string]plugin.Plugin{
		"input": &input.ExternalPlugin{},
	}

	// If we got a checksum, use it
	if checksum != "" {
		checksumraw, err := hex.DecodeString(checksum)
		if err != nil {
			return nil, fmt.Errorf("decoding checksum failed: %v", err)
		}
		securecfg = &plugin.SecureConfig{
			Checksum: checksumraw,
			Hash:     sha256.New(),
		}
	}

	// Startup the client (us) to call functions in the server (the plugin) and receive data
	return plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshake,
		Plugins:          plugins,
		Cmd:              cmd,
		Managed:          true,
		AutoMTLS:         true,
		SecureConfig:     securecfg,
		StartTimeout:     time.Second,
		Logger:           hclog.NewNullLogger(),
		Stderr:           &logger{},
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	}), nil
}

// NewInputWrapper creates a new input plugin wrapper for an external plugin
func NewInputWrapper(name, root, checksum string) inputs.CreatorExternal {
	path := filepath.Join(root, "inputs", name)
	// Return an creator for a creator wrapping the configuration handling
	return func(config string) inputs.CreatorWithError {
		// Welcome to the real inputs.Creator function...
		return func() (telegraf.Input, error) {
			receiver, err := SetupReceiver(exec.Command(path), checksum)
			if err != nil {
				return nil, err
			}
			wrapper, err := input.NewWrapper(name, config, receiver)
			if err != nil {
				return nil, err
			}
			return wrapper, nil
		}
	}
}

// Discover checks the given directory and collects the paths to all executables in the category subdirectories
// it furthermore also checks for a "checksum" file in the directories and parse them
func Discover(root string) (map[string][]string, map[string]map[string]string, error) {
	register := make(map[string][]string)
	checksums := make(map[string]map[string]string)

	for _, category := range []string{"inputs", "outputs", "processors", "aggregators"} {
		categoryPath := filepath.Join(root, category)
		register[category] = make([]string, 0)

		// Check if there is category sub-directory
		fi, err := os.Stat(categoryPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, nil, fmt.Errorf("checking directory %q failed: %v", categoryPath, err)
		}
		if !fi.IsDir() {
			continue
		}

		// Check if there is the "checksum" file
		checksums[category], err = readChecksums(filepath.Join(categoryPath, ChecksumFilename))
		if err != nil {
			return nil, nil, err
		}

		// Walk the category directory and collect all excutables
		err = filepath.Walk(categoryPath, func(path string, info os.FileInfo, err error) error {
			if isFileRegularAndExecutable(path, info) {
				register[category] = append(register[category], filepath.Base(path))
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("path cannot be walked: %v", err)
		}
	}

	return register, checksums, nil
}

func isFileRegularAndExecutable(path string, info os.FileInfo) bool {
	// Symlinks are not followed, but we want to only collect executables. Therefore, call stat,
	// resolving the link for us and gives the actual target properties
	if info.Mode()&os.ModeSymlink != 0 {
		fi, err := os.Stat(path)
		if err != nil {
			log.Printf("W! Bad symbolic link %q", path)
			return false
		}
		info = fi
	}
	// Check if the target has no special file-flags set and is executable
	return info.Mode()&fs.ModeType == 0 && info.Mode()&0111 != 0
}

func readChecksums(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("I! No checksum file found at %q...", filename)
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read checksum file %q: %v", filename, err)
	}
	defer file.Close()

	log.Printf("I! Using checksums in %q...", filename)

	// Read linewise and store the parse out the checksums
	lineno := 0
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineno++
		fields := strings.SplitN(scanner.Text(), " ", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid number of fields in checksum file %q (line %d)", filename, lineno)
		}
		// We need to strip the first character of the field as it denotes text- or binary mode
		checksums[fields[1][1:]] = fields[0]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading checksum file %q failed: %v", filename, err)
	}
	return checksums, nil
}
