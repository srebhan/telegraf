package redistimeseries

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/outputs"
	"github.com/influxdata/telegraf/plugins/parsers/influx"
	"github.com/influxdata/telegraf/testutil"
)

func TestConnectAndWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	address := testutil.GetLocalHost() + ":6379"
	redis := &RedisTimeSeries{
		Address: address,
	}

	// Verify that we can connect to the RedisTimeSeries server
	err := redis.Connect()
	require.NoError(t, err)

	// Verify that we can successfully write data to the RedisTimeSeries server
	err = redis.Write(testutil.MockMetrics())
	require.NoError(t, err)
}

func TestConnectAndWriteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	servicePort := "6379"
	container := testutil.Container{
		Image:        "redislabs/redistimeseries",
		ExposedPorts: []string{servicePort},
		WaitingFor:   wait.ForListeningPort(nat.Port(servicePort)),
	}
	require.NoError(t, container.Start(), "failed to start container")
	defer container.Terminate()
	redis := &RedisTimeSeries{
		Address: fmt.Sprintf("%s:%s", container.Address, container.Ports[servicePort]),
	}
	// Verify that we can connect to the RedisTimeSeries server
	require.NoError(t, redis.Connect())
	// Verify that we can successfully write data to the RedisTimeSeries server
	require.NoError(t, redis.Write(testutil.MockMetrics()))
}

func TestCases(t *testing.T) {
	const servicePort = "6379"
	// Get all testcase directories
	folders, err := os.ReadDir("testcases")
	require.NoError(t, err)

	// Register the plugin
	outputs.Add("redistimeseries", func() telegraf.Output {
		return &RedisTimeSeries{}
	})

	for _, f := range folders {
		// Only handle folders
		if !f.IsDir() {
			continue
		}

		t.Run(f.Name(), func(t *testing.T) {
			testcasePath := filepath.Join("testcases", f.Name())
			configFilename := filepath.Join(testcasePath, "telegraf.conf")
			inputFilename := filepath.Join(testcasePath, "input.influx")
			expectedFilename := filepath.Join(testcasePath, "expected.out")
			expectedErrorFilename := filepath.Join(testcasePath, "expected.err")

			// Get parser to parse input and expected output
			parser := &influx.Parser{}
			require.NoError(t, parser.Init())

			// Load the input data
			input, err := testutil.ParseMetricsFromFile(inputFilename, parser)
			require.NoError(t, err)

			// Read the expected output if any
			var expected []string
			if _, err := os.Stat(expectedFilename); err == nil {
				expected, err = testutil.ParseLinesFromFile(expectedFilename)
				require.NoError(t, err)
			}

			// Read the expected output if any
			var expectedError string
			if _, err := os.Stat(expectedErrorFilename); err == nil {
				expectedErrors, err := testutil.ParseLinesFromFile(expectedErrorFilename)
				require.NoError(t, err)
				require.Len(t, expectedErrors, 1)
				expectedError = expectedErrors[0]
			}

			// Configure the plugin
			cfg := config.NewConfig()
			require.NoError(t, cfg.LoadConfig(configFilename))
			require.Len(t, cfg.Outputs, 1)

			// Setup a test-container
			container := testutil.Container{
				Image:        "redis/redis-stack-server:latest",
				ExposedPorts: []string{servicePort},
				Env:          map[string]string{},
				WaitingFor: wait.ForAll(
					//					wait.ForLog("] mode [basic] - valid"),
					wait.ForListeningPort(nat.Port(servicePort)),
				),
			}
			require.NoError(t, container.Start(), "failed to start container")
			defer container.Terminate()

			address := container.Address + ":" + container.Ports[servicePort]

			// Setup the plugin
			plugin := cfg.Outputs[0].Output.(*RedisTimeSeries)
			plugin.Address = address
			plugin.Log = testutil.Logger{}

			// Connect and write the metric(s)
			require.NoError(t, plugin.Connect())
			defer plugin.Close()

			err = plugin.Write(input)
			if expectedError != "" {
				require.ErrorContains(t, err, expectedError)
				return
			}
			require.NoError(t, err)

			// // Check the metric nevertheless as we might get some metrics despite errors.
			actual := getAllRecords(address)
			require.ElementsMatch(t, expected, actual)
		})
	}
}

func getAllRecords(address string) []string {
	client := redis.NewClient(&redis.Options{Addr: address})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var records []string
	keys := client.Keys(ctx, "*")
	for _, key := range keys.Val() {
		result := client.TSGet(ctx, key)
		records = append(records, fmt.Sprintf("%s: %d=%f", result.Args()[1], result.Val().Timestamp, result.Val().Value))
	}

	return records
}
