package modules

import (
	"time"
	"fmt"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

type DateTime struct {
	gobar.ModuleInterface
	format   string
	location *time.Location
}

func (module *DateTime) InitModule(config gobar.Config) error {
	if format, ok := config["format"].(string); ok {
		module.format = format
	} else {
		module.format = "2006-01-02 15:04:05"
	}
	if location, ok := config["location"].(string); ok {
		zone, err := time.LoadLocation(location)
		if err != nil {
			return fmt.Errorf("Timezone not found: `%s", location)
		} else {
			module.location = zone
		}
	}
	return nil
}

func (module DateTime) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	var now time.Time
	now = time.Now()
	if module.location != nil {
		now = now.In(module.location)
	}

	info.FullText = now.Format(module.format)
	return info
}
func (slot DateTime) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
