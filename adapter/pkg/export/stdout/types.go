package stdout

import (
	"adapter/pkg/common"
	"adapter/pkg/export"
	"encoding/json"
	"fmt"
)

const (
	Status           = "status"
	StartsAt         = "startsAt"
	EndsAt           = "endsAt"
	NotificationTime = "notificationTime"
	RunbookURL       = "runbook_url"
)

type exporter struct {
}

func NewExporter() export.Exporter {

	return &exporter{}
}

func (e *exporter) Export(alerts []*common.Alert) error {

	for _, alert := range alerts {
		fmt.Println(alertToString(alert))
	}

	return nil
}

func (e *exporter) Close() error {
	return nil
}

func alertToString(a *common.Alert) string {

	m := make(map[string]interface{})

	m[Status] = a.Status
	m[StartsAt] = a.StartsAt
	m[EndsAt] = a.EndsAt
	m[NotificationTime] = a.NotificationTime

	for k, v := range a.Labels {
		m[k] = v
	}

	for k, v := range a.Annotations {
		if k != RunbookURL {
			m[k] = v
		}
	}

	bs, err := json.Marshal(m)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return string(bs)
}
