package modules

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Ak-Army/i3barfeeder/gobar"
	"strconv"
	"os/exec"
)

type CpuInfo struct {
	gobar.ModuleInterface
	path      string
	barConfig barConfig
}

var prevTotal, prevIdle uint64

func (module *CpuInfo) InitModule(config gobar.Config) error {
	module.path = keyExists(config, "path", reflect.String, "/").(string)
	module.barConfig.barSize = keyExists(config, "barSize", reflect.Int, 10).(int)
	module.barConfig.barFull = keyExists(config, "barFull", reflect.String, "■").(string)
	module.barConfig.barEmpty = keyExists(config, "barEmpty", reflect.String, "□").(string)

	return nil
}

func (module CpuInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	cpuUsage := module.CpuInfo()
	info.ShortText = fmt.Sprintf("%d %s", int(cpuUsage), "%")
	info.FullText = makeBar(cpuUsage, module.barConfig)
	return info
}
func (module CpuInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	split := strings.Split("gnome-system-monitor -p", " ")
	return nil, exec.Command(split[0], split[1:]...).Start()
}

func (module CpuInfo) CpuInfo() (cpuUsage float64) {
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

