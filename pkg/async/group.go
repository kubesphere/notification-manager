package async

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubesphere/notification-manager/pkg/utils"
)

// Group has several workers, and the group can execute these workers concurrently,
// wait for the workers to finish within a specified time, and receive the results returned by these workers.
type Group struct {
	workers []func(stopCh chan interface{})
	stopCh  chan interface{}
	ctx     context.Context
}

func NewGroup(ctx context.Context) *Group {
	return &Group{
		ctx: ctx,
	}
}

// Add a worker to group
func (g *Group) Add(w func(stopCh chan interface{})) {
	g.workers = append(g.workers, w)
}

// Wait execute all workers concurrently, and wait for all workers to end.
func (g *Group) Wait() error {

	if len(g.workers) == 0 {
		return nil
	}

	g.stopCh = make(chan interface{}, len(g.workers))

	for _, worker := range g.workers {
		go worker(g.stopCh)
	}

	var errs []error
	res := 0
	for {
		select {
		case <-g.ctx.Done():
			return utils.Error("timeout")
		case val := <-g.stopCh:
			switch val.(type) {
			case error:
				errs = append(errs, val.(error))
			case []error:
				errs = append(errs, val.([]error)...)
			default:
			}

			res = res + 1
			if res == len(g.workers) {
				if len(errs) == 0 {
					return nil
				}

				s := ""
				for _, err := range errs {
					s = fmt.Sprintf("%s%s,", s, err.Error())
				}
				return utils.Error(strings.TrimSuffix(s, ","))
			}
		}
	}
}
