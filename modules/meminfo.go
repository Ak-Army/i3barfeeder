package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Ak-Army/config"
	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("MemInfo", func() gobar.ModuleInterface {
		return &MemInfo{
			barConfig: defaultBarConfig(),
		}
	})
}

type MemInfo struct {
	gobar.BaseModule
	barConfig barConfig
}

func (m *MemInfo) InitModule(c *config.SubConfig, log xlog.Logger) error {
	if c != nil {
		return c.Load(&m.barConfig)
	}
	return nil
}

func (m *MemInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	free, total := m.memInfo()
	if total == 0 {
		info.ShortText = "N/A"
		info.FullText = "N/A"
		return info
	}
	freePercent := 100 - 100*(free/total)
	info.ShortText = fmt.Sprintf("%d %s", int(freePercent), "%")
	info.FullText = makeBar(freePercent, m.barConfig)

	return info
}

func (m *MemInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	split := strings.Split("gnome-system-monitor -r", " ")

	return nil, exec.Command(split[0], split[1:]...).Start()
}

func (m *MemInfo) memInfo() (float64, float64) {
	mem := map[string]float64{
		"MemTotal": 0,
		"MemFree":  0,
		"Buffers":  0,
		"Cached":   0,
	}
	callback := func(line string) bool {
		fields := strings.Split(line, ":")
		if _, ok := mem[fields[0]]; ok {
			var val float64
			fmt.Sscanf(fields[1], "%f", &val)
			mem[fields[0]] = val * 1024
		}
		return true
	}
	readLines("/proc/meminfo", callback)
	return mem["MemFree"] + mem["Buffers"] + mem["Cached"], mem["MemTotal"]
}
