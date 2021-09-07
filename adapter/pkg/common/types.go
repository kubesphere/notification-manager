package common

import (
	"time"

	"github.com/golang/glog"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/template"
)

type Alert struct {
	*template.Alert
	NotificationTime time.Time `json:"notificationTime"`
}

func NewAlerts(data []byte) ([]*Alert, error) {

	var d template.Data

	err := jsoniter.Unmarshal(data, &d)
	if err != nil {
		glog.Errorf("unmarshal failed with:%v,body is: %s", err, string(data))
		return nil, err
	}

	var as []*Alert
	for _, a := range d.Alerts {
		alert := a
		as = append(as, &Alert{
			&alert,
			time.Now(),
		})
	}

	return as, nil
}
