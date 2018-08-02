package modules

import (
	"fmt"
	"time"

	"github.com/Ak-Army/i3barfeeder/gobar"
	"os/exec"
)

type DateTime struct {
	gobar.ModuleInterface
	format      string
	shortFormat string
	location    *time.Location
}

func (module *DateTime) InitModule(config gobar.Config) error {
	if format, ok := config["format"].(string); ok {
		module.format = format
	} else {
		module.format = "2006-01-02 15:04:05"
	}
	if shortFormat, ok := config["shortFormat"].(string); ok {
		module.shortFormat = shortFormat
	} else {
		module.shortFormat = "02 15:04:05"
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
	info.ShortText = now.Format(module.shortFormat)
	return info
}
func (slot DateTime) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, exec.Command("gsimplecal").Run()
}
