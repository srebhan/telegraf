package inputs

import "github.com/influxdata/telegraf"

type Creator func() telegraf.Input

var Inputs = map[string]Creator{}

func Add(name string, creator Creator) {
	Inputs[name] = creator
}

type CreatorExternal func(string) Creator

var InputsExternal = map[string]CreatorExternal{}

func AddExternal(name string, creator CreatorExternal) {
	InputsExternal[name] = creator
}
