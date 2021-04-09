package modules

import (
	"encoding/json"
	"fmt"

	"github.com/Ak-Army/i3barfeeder/gobar"

	"github.com/Ak-Army/xlog"
)

func init() {
	gobar.AddModule("Battery", func() gobar.ModuleInterface {
		return &Battery{
			InterfaceName: "BAT1",
			barConfig:     defaultBarConfig(),
		}
	})
}

type Battery struct {
	gobar.ModuleInterface
	InterfaceName string `json:"interfaceName"`
	barConfig     barConfig
	log           xlog.Logger
	fullEnergy    float64
}

func (m *Battery) InitModule(config json.RawMessage, log xlog.Logger) error {
	m.log = log
	if config != nil {
		if err := json.Unmarshal(config, m); err != nil {
			return err
		}
		if err := json.Unmarshal(config, &m.barConfig); err != nil {
			return err
		}
	}
	m.fullEnergy = m.readEnergy("energy_full")

	return nil
}

func (m *Battery) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	currEnergy := m.readEnergy("energy_now")
	freePercent := 100 * (currEnergy / m.fullEnergy)
	m.log.Infof("%f / %f = %f", currEnergy, m.fullEnergy, freePercent)

	info.ShortText = fmt.Sprintf("%d %s", int(freePercent), "%")
	info.FullText = makeBar(freePercent, m.barConfig)
	return info
}

func (m *Battery) readEnergy(name string) float64 {
	var energy float64
	callback := func(line string) bool {
		fmt.Sscanf(line, "%f", &energy)
		return true
	}
	readLines("/sys/class/power_supply/"+m.InterfaceName+"/"+name, callback)
	return energy
}

func (m Battery) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
