package modules

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Ak-Army/config"
	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("CpuInfo", func() gobar.ModuleInterface {
		return &CpuInfo{
			barConfig: defaultBarConfig(),
		}
	})
}

type CpuInfo struct {
	gobar.BaseModule
	barConfig barConfig
	prevTotal uint64
	prevIdle  uint64
}

func (m *CpuInfo) InitModule(c *config.SubConfig, log xlog.Logger) error {
	if c != nil {
		if err := c.Load(m); err != nil {
			return err
		}
		return c.Load(&m.barConfig)
	}
	return nil
}

func (m *CpuInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	cpuUsage := m.CpuInfo()
	info.ShortText = fmt.Sprintf("%d %s", int(cpuUsage), "%")
	info.FullText = makeBar(cpuUsage, m.barConfig)
	return info
}
func (m *CpuInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	split := strings.Split("gnome-system-monitor -p", " ")
	return nil, exec.Command(split[0], split[1:]...).Start()
}

func (m *CpuInfo) CpuInfo() (cpuUsage float64) {
	// Return the percent utilization of the CPU.
	var idle, total uint64
	callback := func(line string) bool {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return true
		}
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, _ := strconv.ParseUint(fields[i], 10, 64)
				total += val
				if i == 4 {
					idle = val
				}
			}
			return false
		}
		return true
	}
	readLines("/proc/stat", callback)

	if m.prevIdle > 0 {
		idleTicks := float64(delta(idle, m.prevIdle))
		totalTicks := float64(delta(total, m.prevTotal))
		if totalTicks > 0 {
			cpuUsage = 100 * (totalTicks - idleTicks) / totalTicks
		}
	}
	m.prevIdle = idle
	m.prevTotal = total
	return
}
