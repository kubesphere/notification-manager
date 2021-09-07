package stdout

import (
	"adapter/pkg/common"
	"adapter/pkg/export"
	"fmt"

	jsoniter "github.com/json-iterator/go"
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

	m := make(map[string]string)

	m[Status] = a.Status
	m[StartsAt] = a.StartsAt.Local().String()
	m[EndsAt] = a.EndsAt.Local().String()
	m[NotificationTime] = a.NotificationTime.Local().String()

	for k, v := range a.Labels {
		m[k] = v
	}

	for k, v := range a.Annotations {
		if k != RunbookURL {
			m[k] = v
		}
	}

	bs, err := jsoniter.Marshal(m)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return string(bs)
}
