package export

import "adapter/pkg/common"

type Exporter interface {
	Export(alerts []*common.Alert) error
	Close() error
}
