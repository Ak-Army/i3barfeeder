package modules

import (
	"github.com/Ak-Army/config"
	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("StaticText", func() gobar.ModuleInterface {
		return &StaticText{}
	})
}

type StaticText struct {
	gobar.BaseModule
}

func (slot *StaticText) InitModule(c *config.SubConfig, log xlog.Logger) error {
	return nil
}

func (slot *StaticText) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	return info
}

func (slot *StaticText) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
