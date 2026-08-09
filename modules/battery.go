package modules

import (
	"fmt"
	"os"
	"strings"

	"github.com/Ak-Army/config"

	"github.com/Ak-Army/i3barfeeder/gobar"

	"github.com/Ak-Army/xlog"
)

func init() {
	gobar.AddModule("Battery", func() gobar.ModuleInterface {
		return &Battery{
			InterfaceName: "BAT0",
			barConfig:     defaultBarConfig(),
		}
	})
}

type Battery struct {
	gobar.BaseModule
	InterfaceName string `config:"interfaceName"`
	barConfig     barConfig
	log           xlog.Logger
	fullEnergy    float64
	nowFile       string
}

func (m *Battery) InitModule(c *config.SubConfig, log xlog.Logger) error {
	m.log = log
	if c != nil {
		if err := c.Load(m); err != nil {
			return err
		}
		if err := c.Load(&m.barConfig); err != nil {
			return err
		}
	}
	// Which pair of counters exists depends on the driver: energy_* on some
	// laptops, charge_* on others, and only capacity on the rest.
	for _, name := range []string{"energy_full", "charge_full"} {
		if m.fullEnergy = m.readEnergy(name); m.fullEnergy > 0 {
			m.nowFile = strings.TrimSuffix(name, "_full") + "_now"
			break
		}
	}

	return nil
}

func (m *Battery) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	var freePercent float64
	switch {
	case m.fullEnergy > 0:
		freePercent = 100 * (m.readEnergy(m.nowFile) / m.fullEnergy)
	case m.hasFile("capacity"):
		freePercent = m.readEnergy("capacity")
	default:
		info.ShortText = "N/A"
		info.FullText = "N/A"
		return info
	}

	info.ShortText = fmt.Sprintf("%d %s", int(freePercent), "%")
	info.FullText = makeBar(freePercent, m.barConfig)
	return info
}

func (m *Battery) hasFile(name string) bool {
	_, err := os.Stat("/sys/class/power_supply/" + m.InterfaceName + "/" + name)
	return err == nil
}

func (m *Battery) readEnergy(name string) float64 {
	if !m.hasFile(name) {
		return 0
	}
	var energy float64
	callback := func(line string) bool {
		fmt.Sscanf(line, "%f", &energy)
		return true
	}
	readLines("/sys/class/power_supply/"+m.InterfaceName+"/"+name, callback)
	return energy
}

func (m *Battery) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
