package rollbar

import "strconv"

type Event interface {
	Tags() map[string]string
	Fields() map[string]interface{}
}

type DummyEvent struct {
	EventName string `json:"event_name"`
}

type NewItemDataItemLastOccurrence struct {
	Language string `json:"language"`
	Level    string `json:"level"`
}

type NewItemDataItem struct {
	ID             int                           `json:"id"`
	Environment    string                        `json:"environment"`
	ProjectID      int                           `json:"project_id"`
	LastOccurrence NewItemDataItemLastOccurrence `json:"last_occurrence"`
}

type NewItemData struct {
	Item NewItemDataItem `json:"item"`
}

type NewItem struct {
	EventName string      `json:"event_name"`
	Data      NewItemData `json:"data"`
}

func (ni *NewItem) Tags() map[string]string {
	return map[string]string{
		"event":       ni.EventName,
		"environment": ni.Data.Item.Environment,
		"project_id":  strconv.Itoa(ni.Data.Item.ProjectID),
		"language":    ni.Data.Item.LastOccurrence.Language,
		"level":       ni.Data.Item.LastOccurrence.Level,
	}
}

func (ni *NewItem) Fields() map[string]interface{} {
	return map[string]interface{}{
		"id": ni.Data.Item.ID,
	}
}

type OccurrenceDataOccurrence struct {
	Language string `json:"language"`
	Level    string `json:"level"`
}

type OccurrenceDataItem struct {
	ID          int    `json:"id"`
	Environment string `json:"environment"`
	ProjectID   int    `json:"project_id"`
}

type OccurrenceData struct {
	Item       OccurrenceDataItem       `json:"item"`
	Occurrence OccurrenceDataOccurrence `json:"occurrence"`
}

type Occurrence struct {
	EventName string         `json:"event_name"`
	Data      OccurrenceData `json:"data"`
}

func (o *Occurrence) Tags() map[string]string {
	return map[string]string{
		"event":       o.EventName,
		"environment": o.Data.Item.Environment,
		"project_id":  strconv.Itoa(o.Data.Item.ProjectID),
		"language":    o.Data.Occurrence.Language,
		"level":       o.Data.Occurrence.Level,
	}
}

func (o *Occurrence) Fields() map[string]interface{} {
	return map[string]interface{}{
		"id": o.Data.Item.ID,
	}
}

type DeployDataDeploy struct {
	ID          int    `json:"id"`
	Environment string `json:"environment"`
	ProjectID   int    `json:"project_id"`
}

type DeployData struct {
	Deploy DeployDataDeploy `json:"deploy"`
}

type Deploy struct {
	EventName string     `json:"event_name"`
	Data      DeployData `json:"data"`
}

func (ni *Deploy) Tags() map[string]string {
	return map[string]string{
		"event":       ni.EventName,
		"environment": ni.Data.Deploy.Environment,
		"project_id":  strconv.Itoa(ni.Data.Deploy.ProjectID),
	}
}

func (ni *Deploy) Fields() map[string]interface{} {
	return map[string]interface{}{
		"id": ni.Data.Deploy.ID,
	}
}
