package gnmilistener

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	common_tls "github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/gnmi_listener/nokia"
	"github.com/influxdata/telegraf/plugins/parsers/influx"
	"github.com/influxdata/telegraf/testutil"
)

func TestCases(t *testing.T) {
	// Get all testcase directories
	folders, err := os.ReadDir("testcases")
	require.NoError(t, err)

	// Register the plugin
	inputs.Add("gnmi", func() telegraf.Input {
		return &GNMIListener{}
	})

	for _, f := range folders {
		// Only handle folders
		if !f.IsDir() {
			continue
		}

		t.Run(f.Name(), func(t *testing.T) {
			testcasePath := filepath.Join("testcases", f.Name())
			configFilename := filepath.Join(testcasePath, "telegraf.conf")
			inputFilename := filepath.Join(testcasePath, "responses.json")
			expectedFilename := filepath.Join(testcasePath, "expected.out")
			expectedErrorFilename := filepath.Join(testcasePath, "expected.err")
			clientConfigFilename := filepath.Join(testcasePath, "client.conf")

			// Load the input data
			buf, err := os.ReadFile(inputFilename)
			require.NoError(t, err)
			var entries []json.RawMessage
			require.NoError(t, json.Unmarshal(buf, &entries))
			responses := make([]*gnmi.SubscribeResponse, 0, len(entries))
			for _, entry := range entries {
				var r gnmi.SubscribeResponse
				require.NoError(t, protojson.Unmarshal(entry, &r))
				responses = append(responses, &r)
			}

			// Prepare the influx parser for expectations
			parser := &influx.Parser{}
			require.NoError(t, parser.Init())

			// Read the expected output if any
			var expected []telegraf.Metric
			if _, err := os.Stat(expectedFilename); err == nil {
				var err error
				expected, err = testutil.ParseMetricsFromFile(expectedFilename, parser)
				require.NoError(t, err)
			}

			// Read the expected output if any
			var expectedErrors []string
			if _, err := os.Stat(expectedErrorFilename); err == nil {
				var err error
				expectedErrors, err = testutil.ParseLinesFromFile(expectedErrorFilename)
				require.NoError(t, err)
				require.NotEmpty(t, expectedErrors)
			}

			// Load the configuration for the client simulating the device
			var clientCfg deviceConfig
			if _, err := os.Stat(clientConfigFilename); err == nil {
				buf, err := os.ReadFile(clientConfigFilename)
				require.NoError(t, err)
				require.NoError(t, toml.Unmarshal(buf, &clientCfg))
			}

			// Configure and setup the plugin
			cfg := config.NewConfig()
			require.NoError(t, cfg.LoadConfig(configFilename))
			require.Len(t, cfg.Inputs, 1)

			plugin := cfg.Inputs[0].Input.(*GNMIListener)
			plugin.Address = "127.0.0.1:0"
			plugin.Log = testutil.Logger{}

			// Start the plugin
			var acc testutil.Accumulator
			require.NoError(t, plugin.Init())
			require.NoError(t, plugin.Start(&acc))
			defer plugin.Stop()

			// Setup a client to mimic the device
			dev, err := newDevice(plugin.server.Address(), plugin.Protocol, &clientCfg)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				dev.start(ctx)
			}()

			// Send the data
			for _, r := range responses {
				dev.send(r)
			}

			// Wait for the metrics to arrive
			require.Eventually(t,
				func() bool {
					return acc.NMetrics() >= uint64(len(expected))
				}, 15*time.Second, 100*time.Millisecond)
			plugin.Stop()
			cancel()
			wg.Wait()

			// Check for errors
			require.Len(t, acc.Errors, len(expectedErrors))
			if len(acc.Errors) > 0 {
				actualErrorMsgs := make([]string, 0, len(acc.Errors))
				for _, err := range acc.Errors {
					actualErrorMsgs = append(actualErrorMsgs, err.Error())
				}
				require.ElementsMatch(t, actualErrorMsgs, expectedErrors)
			}

			// Check the metric nevertheless as we might get some metrics despite errors.
			actual := acc.GetTelegrafMetrics()
			testutil.RequireMetricsEqual(t, expected, actual, testutil.SortMetrics())
		})
	}
}

// Internal functionality

type deviceConfig struct {
	common_tls.ClientConfig
}

type device interface {
	start(context.Context) error
	send(*gnmi.SubscribeResponse)
	errors() []error
	responses() []*gnmi.SubscribeRequest
}

func newDevice(addr, protocol string, cfg *deviceConfig) (device, error) {
	switch protocol {
	case "nokia":
		tlscfg, err := cfg.ClientConfig.TLSConfig()
		if err != nil {
			return nil, fmt.Errorf("creating client TLS failed: %w", err)
		}
		return &nokiaDevice{
			addr:   addr,
			msg:    make(chan *gnmi.SubscribeResponse, 1),
			tlscfg: tlscfg,
		}, nil
	}
	return nil, fmt.Errorf("unknown protocol %q", protocol)
}

type nokiaDevice struct {
	addr   string
	tlscfg *tls.Config

	msg   chan *gnmi.SubscribeResponse
	errs  []error
	resps []*gnmi.SubscribeRequest
	sync.Mutex
}

func (d *nokiaDevice) start(ctx context.Context) error {
	var creds credentials.TransportCredentials

	// Setup the connection credentials
	if d.tlscfg == nil {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(d.tlscfg)
	}

	// Connect to the server
	conn, err := grpc.NewClient(d.addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("dialing server %q failed: %w", d.addr, err)
	}

	// Create a nokia dial-out client
	client := nokia.NewDialoutTelemetryClient(conn)
	stream, err := client.Publish(ctx)
	if err != nil {
		return fmt.Errorf("creating Nokia dial-out client failed: %w", err)
	}

	// Start goroutine to send data and wait for the response
	defer func() {
		if err := stream.CloseSend(); err != nil {
			d.Lock()
			d.errs = append(d.errs, err)
			d.Unlock()
		}
	}()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ctx.Done():
			return nil
		case msg := <-d.msg:
			// Send the message
			if err := stream.Send(msg); err != nil {
				d.Lock()
				d.errs = append(d.errs, err)
				d.Unlock()
				continue
			}

			// Wait for the response
			resp, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				d.Lock()
				d.errs = append(d.errs, err)
				d.Unlock()
				continue
			}

			d.Lock()
			d.resps = append(d.resps, resp)
			d.Unlock()
		}
	}
}

func (d *nokiaDevice) send(msg *gnmi.SubscribeResponse) {
	d.msg <- msg
}

func (d *nokiaDevice) errors() []error {
	d.Lock()
	defer d.Unlock()
	return d.errs
}

func (d *nokiaDevice) responses() []*gnmi.SubscribeRequest {
	d.Lock()
	defer d.Unlock()
	return d.resps
}

/*
   package main

   import (

   	"crypto/ecdsa"
   	"crypto/elliptic"
   	"crypto/rand"
   	"crypto/tls"
   	"crypto/x509"
   	"encoding/pem"
   	"fmt"
   	"math/big"
   	"net"
   	"time"

   	"google.golang.org/grpc"
   	"google.golang.org/grpc/connectivity"
   	"google.golang.org/grpc/credentials"

   )

   	func check(err error) {
   		if err != nil {
   			panic(err)
   		}
   	}

   	func main() {
   		lis, err := net.Listen("tcp", "127.0.0.1:18080")
   		check(err)

   		certPem, privPem := GenerateCertAndKeys()

   		pool := x509.NewCertPool()
   		pool.AppendCertsFromPEM(certPem)

   		pair, err := tls.X509KeyPair(certPem, privPem)
   		tc := credentials.NewTLS(&tls.Config{
   			Certificates: []tls.Certificate{pair},
   			ClientAuth:   tls.RequireAndVerifyClientCert,
   			ClientCAs:    pool,
   			RootCAs:      pool,
   		})

   		server := grpc.NewServer(grpc.Creds(tc))

   		go server.Serve(lis)

   		conn, err := grpc.Dial("127.0.0.1:18080", grpc.WithTransportCredentials(tc))
   		check(err)

   		defer func() {
   			err = conn.Close()
   			check(err)
   			server.Stop()
   		}()

   		for {
   			switch state := conn.GetState(); state {
   			case connectivity.Ready:
   				fmt.Printf("Connection established!\n")
   				return
   			case connectivity.TransientFailure:
   				fmt.Printf("Failed to connect...%s\n", state)
   				return
   			default:
   				fmt.Printf("STATE = %s\n", state)
   			}
   			time.Sleep(time.Second)
   		}
   	}

   	func GenerateCertAndKeys() (c, k []byte) {
   		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
   		check(err)
   		privDer, err := x509.MarshalPKCS8PrivateKey(priv)
   		check(err)
   		privPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDer})

   		template := &x509.Certificate{
   			SerialNumber: new(big.Int),
   			NotAfter:     time.Now().Add(time.Hour),
   			IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
   		}

   		certDer, err := x509.CreateCertificate(rand.Reader, template, template, priv.Public(), priv)
   		check(err)
   		certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDer})
   		check(err)

   		return certPem, privPem
   	}
*/
