//go:build (!custom || inputs || inputs.win_services) && windows

package all

import _ "github.com/influxdata/telegraf/plugins/inputs/win_services" // register plugin
