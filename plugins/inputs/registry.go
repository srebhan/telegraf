package inputs

import "github.com/influxdata/telegraf"

type Creator func() telegraf.Input
type CreatorWithError func() (telegraf.Input, error)

var Inputs = map[string]CreatorWithError{}

func Add(name string, creator Creator) {
	Inputs[name] = func() (telegraf.Input, error) { return creator(), nil }
}

type CreatorExternal func(string) CreatorWithError

var InputsExternal = map[string]CreatorExternal{}

func AddExternal(name string, creator CreatorExternal) {
	InputsExternal[name] = creator
}
