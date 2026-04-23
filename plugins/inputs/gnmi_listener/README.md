# gNMI (gRPC Network Management Interface) dial-out Input Plugin

This plugin consumes telemetry data based on [gNMI][gnmi] messages sent by
network devices in dial-out mode. This plugin supports a list of vendor
protocols such as [Nokia dial-out telemetry][nokia].

⭐ Telegraf v1.39.0
🏷️ network
💻 all

[gnmi]: https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-specification.md
[nokia]: https://infocenter.nokia.com/public/7750SR222R1A/index.jsp?topic=%2Fcom.nokia.System_Mgmt_Guide%2Fdial-out_teleme-ai9exj5ye3.html

## Service Input <!-- @/docs/includes/service_input.md -->

This plugin is a service input. Normal plugins gather metrics determined by the
interval setting. Service plugins start a service to listen and wait for
metrics or events to occur. Service plugins have two key differences from
normal plugins:

1. The global or plugin specific `interval` setting may not apply
2. The CLI options of `--test`, `--test-wait`, and `--once` may not produce
   output for this plugin

## Global configuration options <!-- @/docs/includes/plugin_config.md -->

Plugins support additional global and plugin configuration settings for tasks
such as modifying metrics, tags, and fields, creating aliases, and configuring
plugin ordering. See [CONFIGURATION.md][CONFIGURATION.md] for more details.

[CONFIGURATION.md]: ../../../docs/CONFIGURATION.md#plugins

## Configuration

```toml @sample.conf
# gNMI dial-out telemetry plugin
[[inputs.gnmi_listener]]
  ## Address and port of the gNMI GRPC server
  address = "localhost:57400"

  ## Protocol to use, available options:
  ##   nokia -- Nokia SR OS dial-out protocol
  # protocol = "nokia"

  ## Emit a metric for "delete" messages
  # emit_delete_metrics = false

  ## Enable to get the canonical path as field-name
  # canonical_field_names = false

  ## Remove leading slashes and dots in field-name
  # trim_field_names = false

  ## Prefix tags from path keys with the path element
  # prefix_tag_key_with_path = false

  ## Guess the path-tag if an update does not contain a prefix-path
  ## Supported values are
  ##   none         -- do not add a 'path' tag
  ##   common path  -- use the common path elements of all fields in an update
  ##   subscription -- use the subscription path
  # path_guessing_strategy = "none"

  ## Enforces the namespace of the first element as origin for aliases and
  ## response paths, required for backward compatibility.
  ## NOTE: Set to 'false' if possible but be aware that this might change the path tag!
  # enforce_first_namespace_as_origin = true

  ## Vendor specific options
  ## This defines what vendor specific options to load.
  ## * Juniper Header Extension (juniper_header): some sensors are directly managed by
  ##   Linecard, which adds the Juniper GNMI Header Extension. Enabling this
  ##   allows the decoding of the Extension header if present. Currently this knob
  ##   adds component, component_id & sub_component_id as additional tags
  # vendor_specific = []

  ## YANG model paths for decoding IETF JSON payloads
  ## Model files are loaded recursively from the given directories. Disabled if
  ## no models are specified.
  # yang_model_paths = []
```

## Metrics

Each configured subscription will emit a different measurement. Each leaf in a
GNMI SubscribeResponse Update message will produce a field reading in the
measurement. GNMI PathElement keys for leaves will attach tags to the field(s).

## Example Output

```text
ifcounters,path=openconfig-interfaces:/interfaces/interface/state/counters,host=linux,name=MgmtEth0/RP0/CPU0/0,source=10.49.234.115,descr/description=Foo in-multicast-pkts=0i,out-multicast-pkts=0i,out-errors=0i,out-discards=0i,in-broadcast-pkts=0i,out-broadcast-pkts=0i,in-discards=0i,in-unknown-protos=0i,in-errors=0i,out-unicast-pkts=0i,in-octets=0i,out-octets=0i,last-clear="2019-05-22T16:53:21Z",in-unicast-pkts=0i 1559145777425000000
ifcounters,path=openconfig-interfaces:/interfaces/interface/state/counters,host=linux,name=GigabitEthernet0/0/0/0,source=10.49.234.115,descr/description=Bar out-multicast-pkts=0i,out-broadcast-pkts=0i,in-errors=0i,out-errors=0i,in-discards=0i,out-octets=0i,in-unknown-protos=0i,in-unicast-pkts=0i,in-octets=0i,in-multicast-pkts=0i,in-broadcast-pkts=0i,last-clear="2019-05-22T16:54:50Z",out-unicast-pkts=0i,out-discards=0i 1559145777425000000
```

## Troubleshooting

### Missing `path` tag

Some devices (e.g. Arista) omit the prefix and specify the path in the update
if there is only one value reported. This leads to a missing `path` tag for
the resulting metrics. In those cases you should set `path_guessing_strategy`
to `subscription` to use the subscription path as `path` tag.

Other devices might omit the prefix in updates altogether. Here setting
`path_guessing_strategy` to `common path` can help to infer the `path` tag by
using the part of the path that is common to all values in the update.

<!--
### TLS handshake failure

When receiving an error like

```text
2024-01-01T00:00:00Z E! [inputs.gnmi] Error in plugin: failed to setup subscription: rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: remote error: tls: handshake failure"
```

this might be due to insecure TLS configurations in the GNMI server. Please
check the minimum TLS version provided by the server as well as the cipher suite
used. You might want to use the `tls_min_version` or `tls_cipher_suites` setting
respectively to work-around the issue. Please be careful to not undermine the
security of the connection between the plugin and the device!

-->