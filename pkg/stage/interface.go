package stage

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/modern-go/reflect2"
)

// A Stage processes alerts under the constraints of the given context.
type Stage interface {
	Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error)
}

// A MultiStage executes a series of stages sequentially.
type MultiStage []Stage

// Exec implements the Stage interface.
func (ms MultiStage) Exec(ctx context.Context, l log.Logger, data interface{}) (context.Context, interface{}, error) {
	var err error
	for _, s := range ms {
		if reflect2.IsNil(data) {
			return ctx, nil, nil
		}

		ctx, data, err = s.Exec(ctx, l, data)
		if err != nil {
			return ctx, nil, err
		}
	}
	return ctx, data, nil
}
