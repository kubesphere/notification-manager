package memory

import (
	"context"
	"time"

	"github.com/kubesphere/notification-manager/pkg/store/provider"
	"github.com/kubesphere/notification-manager/pkg/template"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	queueLen    *int
	pushTimeout *time.Duration
)

type memProvider struct {
	ch chan *template.Alert
}

func init() {
	queueLen = kingpin.Flag(
		"store.memory.queue",
		"Memory cache queue capacity",
	).Default("10000").Int()
	pushTimeout = kingpin.Flag(
		"store.memory.pushTimeout",
		"Push timeout",
	).Default("3s").Duration()
}

func NewProvider() provider.Provider {

	return &memProvider{
		ch: make(chan *template.Alert, *queueLen),
	}
}

func (p *memProvider) Push(alert *template.Alert) error {
	ctx, cancel := context.WithTimeout(context.Background(), *pushTimeout)
	defer cancel()

	select {
	case p.ch <- alert:
		return nil
	case <-ctx.Done():
		return utils.Error("Time out")
	}
}

func (p *memProvider) Pull(batchSize int, batchWait time.Duration) ([]*template.Alert, error) {

	ctx, cancel := context.WithTimeout(context.Background(), batchWait)
	defer cancel()

	var as []*template.Alert
	for {
		select {
		case <-ctx.Done():
			return as, nil
		case alert := <-p.ch:
			if alert == nil {
				return as, utils.Error("Store closed")
			}
			as = append(as, alert)
			if len(as) >= batchSize {
				return as, nil
			}
		}
	}
}

func (p *memProvider) Close() error {
	close(p.ch)
	return nil
}
