package modules

import (
	"github.com/Ak-Army/i3barfeeder/gobar"
)

type StaticText struct {
	gobar.ModuleInterface
}

func (slot *StaticText) InitModule(config gobar.Config) error {
	return nil
}

func (slot StaticText) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	return info
}

func (slot StaticText) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
