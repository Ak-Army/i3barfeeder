package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("DateTime", func() gobar.ModuleInterface {
		return &DateTime{
			Format:      "2006-01-02 15:04:05",
			ShortFormat: "02 15:04:05",
		}
	})
}

type DateTime struct {
	gobar.ModuleInterface
	Format      string `json:"format"`
	ShortFormat string `json:"shortFormat"`
	Location    string `json:"location"`
	location    *time.Location
}

func (m *DateTime) InitModule(config json.RawMessage, log xlog.Logger) error {
	if config != nil {
		if err := json.Unmarshal(config, m); err != nil {
			return err
		}
	}
	if m.Format == "" {
		m.Format = "2006-01-02 15:04:05"
	}
	if m.ShortFormat == "" {
		m.ShortFormat = "02 15:04:05"
	}
	if m.Location != "" {
		zone, err := time.LoadLocation(m.Location)
		if err != nil {
			return fmt.Errorf("timezone not found: `%s", m.Location)
		}
		m.location = zone
	}
	return nil
}

func (m DateTime) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	var now time.Time
	now = time.Now()
	if m.location != nil {
		now = now.In(m.location)
	}

	info.FullText = now.Format(m.Format)
	info.ShortText = now.Format(m.ShortFormat)
	return info
}
func (m DateTime) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, exec.Command("gsimplecal").Run()
}
