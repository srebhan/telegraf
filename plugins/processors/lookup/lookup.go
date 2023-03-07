//go:generate ../../../tools/readme_config_includer/generator
package lookup

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"
)

//go:embed sample.conf
var sampleConfig string

type Processor struct {
	Filenames   []string        `toml:"files"`
	Fileformat  string          `toml:"format"`
	KeyTemplate string          `toml:"key"`
	Log         telegraf.Logger `toml:"-"`

	tmpl     *template.Template
	mappings map[string][]telegraf.Tag
}

func (*Processor) SampleConfig() string {
	return sampleConfig
}

func (p *Processor) Init() error {
	if len(p.Filenames) < 1 {
		return errors.New("missing 'files'")
	}

	if p.KeyTemplate == "" {
		return errors.New("missing 'key_template'")
	}

	tmpl, err := template.New("key").Parse(p.KeyTemplate)
	if err != nil {
		return fmt.Errorf("creating template failed: %w", err)
	}
	p.tmpl = tmpl

	p.mappings = make(map[string][]telegraf.Tag)
	switch strings.ToLower(p.Fileformat) {
	case "", "json":
		return p.loadJsonFiles()
	case "csv_key_name_value":
		return p.loadCSVKeyNameValueFiles()
	}

	return fmt.Errorf("invalid format %q", p.Fileformat)
}

func (p *Processor) Apply(in ...telegraf.Metric) []telegraf.Metric {
	out := make([]telegraf.Metric, 0, len(in))
	for _, m := range in {
		var buf bytes.Buffer
		if err := p.tmpl.Execute(&buf, m); err != nil {
			p.Log.Errorf("generating key failed: %v", err)
			p.Log.Debugf("metric was %v", m)
			out = append(out, m)
			continue
		}

		if tags, found := p.mappings[buf.String()]; found {
			for _, tag := range tags {
				m.AddTag(tag.Key, tag.Value)
			}
		}
		out = append(out, m)
	}
	return out
}

func (p *Processor) loadJsonFiles() error {
	for _, fn := range p.Filenames {
		buf, err := os.ReadFile(fn)
		if err != nil {
			return fmt.Errorf("loading %q failed: %w", fn, err)
		}

		var data map[string]map[string]string
		if err := json.Unmarshal(buf, &data); err != nil {
			return fmt.Errorf("parsing %q failed: %w", fn, err)
		}

		for key, tags := range data {
			for k, v := range tags {
				p.mappings[key] = append(p.mappings[key], telegraf.Tag{Key: k, Value: v})
			}
		}
	}
	return nil
}

func (p *Processor) loadCSVKeyNameValueFiles() error {
	for _, fn := range p.Filenames {
		if err := p.loadCSVKeyNameValueFile(fn); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) loadCSVKeyNameValueFile(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("loading %q failed: %w", fn, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comment = '#'
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	line := 0
	for {
		line++
		data, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("reading line %d in %q failed: %w", line, fn, err)
		}
		if len(data) < 3 {
			return fmt.Errorf("line %d in %q has not enough fields, requiring at least `key,name,value`", line, fn)
		}
		if len(data)%2 != 1 {
			return fmt.Errorf("line %d in %q has a tag-name without value", line, fn)
		}

		key := data[0]
		for i := 1; i < len(data)-1; i += 2 {
			k, v := data[i], data[i+1]
			p.mappings[key] = append(p.mappings[key], telegraf.Tag{Key: k, Value: v})
		}
	}

	return nil
}

func init() {
	processors.Add("lookup", func() telegraf.Processor {
		return &Processor{}
	})
}
