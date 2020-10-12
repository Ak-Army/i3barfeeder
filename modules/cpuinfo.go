package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

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
	gobar.ModuleInterface
	barConfig barConfig
}

var prevTotal, prevIdle uint64

func (m *CpuInfo) InitModule(config json.RawMessage, log xlog.Logger) error {
	if config != nil {
		return json.Unmarshal(config, &m.barConfig)
	}
	return nil
}

func (m CpuInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	cpuUsage := m.CpuInfo()
	info.ShortText = fmt.Sprintf("%d %s", int(cpuUsage), "%")
	info.FullText = makeBar(cpuUsage, m.barConfig)
	return info
}
func (m CpuInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	split := strings.Split("gnome-system-monitor -p", " ")
	return nil, exec.Command(split[0], split[1:]...).Start()
}

func (m CpuInfo) CpuInfo() (cpuUsage float64) {
	// Return the percent utilization of the CPU.
	var idle, total uint64
	callback := func(line string) bool {
		fields := strings.Fields(line)
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

	if prevIdle > 0 {
		idleTicks := float64(idle - prevIdle)
		totalTicks := float64(total - prevTotal)
		cpuUsage = 100 * (totalTicks - idleTicks) / totalTicks
	}
	prevIdle = idle
	prevTotal = total
	return
}
