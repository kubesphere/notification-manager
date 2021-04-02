package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/json-iterator/go"
	"github.com/prometheus/alertmanager/template"
	"sync"
	"time"
)

type Alert struct {
	*template.Alert
	*TraceInfo
}

type TraceInfo struct {
	//The time when received the data
	ReceivedTime time.Time
	//The time when push the data to the channel.
	PushTime time.Time
	//The time when the data is pulled from channel
	PullTime time.Time
	//Does the event wait goroutines timeout
	WaitRoutinesTimeout bool
	//The time when the message send completely
	SendTime time.Time
	//Does the message send timeout
	SendTimeout bool
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
			&TraceInfo{ReceivedTime: time.Now()},
		})
	}

	return as, nil
}

type Statistics struct {
	ch chan *Alert

	Enable    bool
	FreshTime time.Duration

	wakeup chan int

	WaitToChanSum              int64
	WaitToChanCount            int
	InChanSum                  int64
	InChanCount                int
	SendSum                    int64
	SendCount                  int
	SendSuccessSum             int64
	SendSuccessCount           int
	WaitGoroutinesTimeoutCount int
	SendTimeoutCount           int

	mu sync.Mutex
}

func NewStatisticsInfo(ch chan *Alert) *Statistics {
	info := &Statistics{
		ch:        ch,
		FreshTime: time.Minute * 5,
		Enable:    true,
	}

	info.wakeup = make(chan int)

	go info.Watch()

	return info
}

func (si *Statistics) Watch() {

	to := time.NewTimer(si.FreshTime)
	for {
		to.Reset(si.FreshTime)
		select {
		case <-si.wakeup:
		case <-to.C:
			si.Refresh()
		}
	}
}

func (si *Statistics) StatisticsStep(a *Alert) {

	if !si.Enable || a == nil {
		return
	}

	si.mu.Lock()
	defer si.mu.Unlock()

	if a.PushTime.Unix() > 0 {
		si.WaitToChanSum += a.PushTime.UnixNano() - a.ReceivedTime.UnixNano()
		si.WaitToChanCount++
	}

	if a.PullTime.Unix() > 0 {
		si.InChanSum += a.PullTime.UnixNano() - a.PushTime.UnixNano()
		si.InChanCount++
	}

	if a.SendTime.Unix() > 0 {
		si.SendSum += a.SendTime.UnixNano() - a.PullTime.UnixNano()
		si.SendCount++
	}

	if a.WaitRoutinesTimeout {
		si.WaitGoroutinesTimeoutCount++
	}

	if a.SendTimeout {
		si.SendTimeoutCount++
	}
}

func (si *Statistics) SetFreshTime(t time.Duration) {
	si.FreshTime = t
	si.wakeup <- 1
}

func (si *Statistics) Refresh() {

	if !si.Enable {
		return
	}

	bs, err := json.Marshal(si.Print())
	if err != nil {
		glog.Error(err)
	} else {
		glog.Error(string(bs))
	}

	si.mu.Lock()
	defer si.mu.Unlock()

	si.WaitToChanSum = 0
	si.WaitToChanCount = 0
	si.InChanSum = 0
	si.InChanCount = 0
	si.SendSum = 0
	si.SendCount = 0
	si.WaitGoroutinesTimeoutCount = 0
	si.SendTimeoutCount = 0
}

func (si *Statistics) Print() map[string]string {

	si.mu.Lock()
	defer si.mu.Unlock()

	m := make(map[string]string)

	if si.Enable {

		m["Length"] = fmt.Sprint(len(si.ch))

		if si.WaitToChanCount > 0 {
			m["WaitToChanSum"] = fmt.Sprintf("%.3fms", float64(si.WaitToChanSum)/1e6)
			m["WaitToChanCount"] = fmt.Sprint(si.WaitToChanCount)
			m["WaitToChanAverage"] = fmt.Sprintf("%.3fms", float64(si.WaitToChanSum)/float64(si.WaitToChanCount)/1e6)
		}
		if si.InChanCount > 0 {
			m["InChanSum"] = fmt.Sprintf("%.3fms", float64(si.InChanSum)/1e6)
			m["InChanCount"] = fmt.Sprint(si.InChanCount)
			m["InChanAverage"] = fmt.Sprintf("%.3fms", float64(si.InChanSum)/float64(si.InChanCount)/1e6)
		}
		if si.SendCount > 0 {
			m["SendSum"] = fmt.Sprintf("%.3fms", float64(si.SendSum)/1e6)
			m["SendCount"] = fmt.Sprint(si.SendCount)
			m["SendAverage"] = fmt.Sprintf("%.3fms", float64(si.SendSum)/float64(si.SendCount)/1e6)
		}
		if si.WaitGoroutinesTimeoutCount > 0 {
			m["WaitGoroutinesTimeoutCount"] = fmt.Sprint(si.WaitGoroutinesTimeoutCount)
		}
		if si.SendTimeoutCount > 0 {
			m["MatchTimeoutCount"] = fmt.Sprint(si.SendTimeoutCount)
		}
	}

	return m
}
