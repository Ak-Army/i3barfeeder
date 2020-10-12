package modules

import (
	"encoding/json"

	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("StaticText", func() gobar.ModuleInterface {
		return &StaticText{}
	})
}

type StaticText struct {
	gobar.ModuleInterface
}

func (slot *StaticText) InitModule(config json.RawMessage, log xlog.Logger) error {
	return nil
}

func (slot StaticText) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	return info
}

func (slot StaticText) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
