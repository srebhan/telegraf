package external

import(
  "errors"
  "fmt"
  "time"

  "github.com/influxdata/telegraf"
  "github.com/influxdata/telegraf/external/protocol"
  "github.com/influxdata/telegraf/internal"
  "github.com/influxdata/telegraf/metric"
)

// ToErrorMessage converts a Golang error to a protobuf error type
func ToErrorMessage(err error) *protocol.Error {
  if err == nil {
    return &protocol.Error{}
  }
  return &protocol.Error{
    IsError: true,
    Message: err.Error(),
  }
}

// FromErrorMessage converts a protobuf error type to a Golang error
func FromErrorMessage(err *protocol.Error) error {
  if !err.GetIsError() {
    return nil
  }

  return errors.New(err.GetMessage())
}

// ToMetricMessage converts a telegraf metric to a protobuf metric message
func ToMetricMessage(metric telegraf.Metric) (*protocol.Metric, error) {
  m := protocol.Metric{
    Name: metric.Name(),
    Tags: make([]*protocol.Tag, 0, len(metric.TagList())),
    Fields: make([]*protocol.Field, 0, len(metric.FieldList())),
    Time: metric.Time().UnixNano(),
  }

  for _, tag := range metric.TagList() {
    t := protocol.Tag{
      Key: tag.Key,
      Value: tag.Value,
    }
    m.Tags = append(m.Tags, &t)
  }

  for _, field := range metric.FieldList() {
    f := protocol.Field{Key: field.Key}
    switch fv := field.Value.(type) {
    case int, int8, int16, int32, int64:
      v, err := internal.ToInt64(field.Value)
      if err != nil {
        return nil, fmt.Errorf("converting %q to int64 failed: %v", field.Key, err)
      }
      f.Type = protocol.FieldType_Int64
      f.Value = &protocol.Field_ValueI64{ValueI64: v}
    case uint, uint8, uint16, uint32, uint64:
      v, err := internal.ToUint64(field.Value)
      if err != nil {
        return nil, fmt.Errorf("converting %q to uint64 failed: %v", field.Key, err)
      }
      f.Type = protocol.FieldType_Uint64
      f.Value = &protocol.Field_ValueU64{ValueU64: v}
    case float32, float64:
      v, err := internal.ToFloat64(field.Value)
      if err != nil {
        return nil, fmt.Errorf("converting %q to float64 failed: %v", field.Key, err)
      }
      f.Type = protocol.FieldType_Float64
      f.Value = &protocol.Field_ValueF64{ValueF64: v}
    case string:
      f.Type = protocol.FieldType_String
      f.Value = &protocol.Field_ValueString{ValueString: fv}
    case bool:
      f.Type = protocol.FieldType_Bool
      f.Value = &protocol.Field_ValueBool{ValueBool: fv}
    }
    m.Fields = append(m.Fields, &f)
  }

  switch metric.Type() {
  case telegraf.Counter:
    m.Type = protocol.ValueType_Counter
  case telegraf.Gauge:
    m.Type = protocol.ValueType_Gauge
  case telegraf.Summary:
    m.Type = protocol.ValueType_Summary
  case telegraf.Histogram:
    m.Type = protocol.ValueType_Histogram
  }

  return &m, nil
}

// FromMetricMessage converts a protobuf metric to a telegraf metric
func FromMetricMessage(m *protocol.Metric) telegraf.Metric {
  name := m.GetName()
  tags := make(map[string]string)
  fields := make(map[string]interface{})
  mtype := telegraf.Untyped
  t := m.GetTime()

  for _, tag := range m.GetTags() {
    tags[tag.GetKey()] = tag.GetValue()
  }

  for _, field := range m.GetFields() {
    var value interface{}

    switch field.GetType() {
    case protocol.FieldType_Int64:
      value = field.GetValueI64()
    case protocol.FieldType_Uint64:
      value = field.GetValueU64()
    case protocol.FieldType_Float64:
      value = field.GetValueF64()
    case protocol.FieldType_String:
      value = field.GetValueString()
    case protocol.FieldType_Bool:
      value = field.GetValueBool()
    }
    fields[field.GetKey()] = value
  }

  switch m.GetType() {
  case protocol.ValueType_Counter:
    mtype = telegraf.Counter
  case protocol.ValueType_Gauge:
    mtype = telegraf.Gauge
  case protocol.ValueType_Summary:
    mtype = telegraf.Summary
  case protocol.ValueType_Histogram:
    mtype = telegraf.Histogram
  }

  return metric.New(name, tags, fields, time.Unix(0, t), mtype)
}

// ToMetricsMessage converts a telegraf metric slice to a protobuf metric array
func ToMetricsMessage(metrics []telegraf.Metric) ([]*protocol.Metric, error) {
  msgmetrics := make([]*protocol.Metric, 0, len(metrics))
  for _, m := range metrics {
    msg, err := ToMetricMessage(m)
    if err != nil {
      return nil, fmt.Errorf("converting metric %v failed: %v", m, err)
    }
    msgmetrics = append(msgmetrics, msg)
  }
  return msgmetrics, nil
}

// FromMetricsMessage converts a protobuf metric array to a telegraf metric slice
func FromMetricsMessage(msgmetrics []*protocol.Metric) []telegraf.Metric {
  metrics := make([]telegraf.Metric, 0, len(msgmetrics))
  for _, m := range msgmetrics {
    metrics = append(metrics, FromMetricMessage(m))
  }
  return metrics
}
