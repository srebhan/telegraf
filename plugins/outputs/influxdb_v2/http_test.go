package influxdb_v2_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/common/limited"
	influxdb "github.com/influxdata/telegraf/plugins/outputs/influxdb_v2"
	"github.com/influxdata/telegraf/plugins/serializers/influx"
	"github.com/influxdata/telegraf/testutil"
)

func genURL(u string) *url.URL {
	//nolint:errcheck // known test urls
	address, _ := url.Parse(u)
	return address
}
func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		err bool
		cfg *influxdb.HTTPConfig
	}{
		{
			err: true,
			cfg: &influxdb.HTTPConfig{},
		},
		{
			err: true,
			cfg: &influxdb.HTTPConfig{
				URL: genURL("udp://localhost:9999"),
			},
		},
		{
			cfg: &influxdb.HTTPConfig{
				URL: genURL("unix://var/run/influxd.sock"),
			},
		},
		{
			cfg: &influxdb.HTTPConfig{
				URL:             genURL("unix://var/run/influxd.sock"),
				PingTimeout:     config.Duration(15 * time.Second),
				ReadIdleTimeout: config.Duration(30 * time.Second),
			},
		},
	}

	for i := range tests {
		client, err := influxdb.NewHTTPClient(tests[i].cfg)
		if !tests[i].err {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			t.Log(err)
		}
		if err == nil {
			client.URL()
		}
	}
}

func TestWrite(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/write":
				err := r.ParseForm()
				require.NoError(t, err)
				require.Equal(t, []string{"foobar"}, r.Form["bucket"])

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), "cpu value=42.123")

				w.WriteHeader(http.StatusNoContent)
				return
			default:
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}),
	)
	defer ts.Close()

	addr := &url.URL{
		Scheme: "http",
		Host:   ts.Listener.Addr().String(),
	}

	serializer := &influx.Serializer{}
	require.NoError(t, serializer.Init())

	cfg := &influxdb.HTTPConfig{
		URL:              addr,
		Bucket:           "telegraf",
		BucketTag:        "bucket",
		ExcludeBucketTag: true,
		PingTimeout:      config.Duration(15 * time.Second),
		ReadIdleTimeout:  config.Duration(30 * time.Second),
		Serializer:       limited.NewIndividualSerializer(serializer),
		Log:              &testutil.Logger{},
	}

	client, err := influxdb.NewHTTPClient(cfg)
	require.NoError(t, err)

	metrics := []telegraf.Metric{
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"bucket": "foobar",
			},
			map[string]interface{}{
				"value": 42.123,
			},
			time.Unix(0, 0),
		),
	}

	ctx := context.Background()
	require.NoError(t, client.Write(ctx, metrics))
	require.NoError(t, client.Write(ctx, metrics))
}

func TestWriteBucketTagWorksOnRetry(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/write":
				err := r.ParseForm()
				require.NoError(t, err)
				require.Equal(t, []string{"foo"}, r.Form["bucket"])

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), "cpu value=42")

				w.WriteHeader(http.StatusNoContent)
				return
			default:
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}),
	)
	defer ts.Close()

	addr := &url.URL{
		Scheme: "http",
		Host:   ts.Listener.Addr().String(),
	}

	serializer := &influx.Serializer{}
	require.NoError(t, serializer.Init())

	cfg := &influxdb.HTTPConfig{
		URL:              addr,
		Bucket:           "telegraf",
		BucketTag:        "bucket",
		ExcludeBucketTag: true,
		Serializer:       limited.NewIndividualSerializer(serializer),
		Log:              &testutil.Logger{},
	}

	client, err := influxdb.NewHTTPClient(cfg)
	require.NoError(t, err)

	metrics := []telegraf.Metric{
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"bucket": "foo",
			},
			map[string]interface{}{
				"value": 42.0,
			},
			time.Unix(0, 0),
		),
	}

	ctx := context.Background()
	require.NoError(t, client.Write(ctx, metrics))
	require.NoError(t, client.Write(ctx, metrics))
}

func TestTooLargeWriteRetry(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/write":
				err := r.ParseForm()
				require.NoError(t, err)

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				// Ensure metric body size is small
				if len(body) > 16 {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
				} else {
					w.WriteHeader(http.StatusNoContent)
				}

				return
			default:
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}),
	)
	defer ts.Close()

	addr := &url.URL{
		Scheme: "http",
		Host:   ts.Listener.Addr().String(),
	}

	serializer := &influx.Serializer{}
	require.NoError(t, serializer.Init())

	cfg := &influxdb.HTTPConfig{
		URL:              addr,
		Bucket:           "telegraf",
		BucketTag:        "bucket",
		ExcludeBucketTag: true,
		Serializer:       limited.NewIndividualSerializer(serializer),
		Log:              &testutil.Logger{},
	}

	client, err := influxdb.NewHTTPClient(cfg)
	require.NoError(t, err)

	// Together the metric batch size is too big, split up, we get success
	metrics := []telegraf.Metric{
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"bucket": "foo",
			},
			map[string]interface{}{
				"value": 42.0,
			},
			time.Unix(0, 0),
		),
		testutil.MustMetric(
			"cpu",
			map[string]string{
				"bucket": "bar",
			},
			map[string]interface{}{
				"value": 99.0,
			},
			time.Unix(0, 0),
		),
	}

	ctx := context.Background()
	require.NoError(t, client.Write(ctx, metrics))

	// These metrics are too big, even after splitting in half, expect error
	hugeMetrics := []telegraf.Metric{
		testutil.MustMetric(
			"reallyLargeMetric",
			map[string]string{
				"bucket": "foobar",
			},
			map[string]interface{}{
				"value": 123.456,
			},
			time.Unix(0, 0),
		),
		testutil.MustMetric(
			"evenBiggerMetric",
			map[string]string{
				"bucket": "fizzbuzzbang",
			},
			map[string]interface{}{
				"value": 999.999,
			},
			time.Unix(0, 0),
		),
	}
	require.Error(t, client.Write(ctx, hugeMetrics))
}
