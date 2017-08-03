package modules

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Ak-Army/i3barfeeder/gobar"
	"os/exec"
)

type MemInfo struct {
	gobar.ModuleInterface
	path      string
	barConfig barConfig
}

func (module *MemInfo) InitModule(config gobar.Config) error {
	module.path = keyExists(config, "path", reflect.String, "/").(string)
	module.barConfig.barSize = keyExists(config, "barSize", reflect.Int, 10).(int)
	module.barConfig.barFull = keyExists(config, "barFull", reflect.String, "■").(string)
	module.barConfig.barEmpty = keyExists(config, "barEmpty", reflect.String, "□").(string)

	return nil
}

func (module MemInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	free, total := module.memInfo()
	freePercent := 100 - 100*(free/total)
	info.ShortText = fmt.Sprintf("%d %s", int(freePercent), "%")
	info.FullText = makeBar(freePercent, module.barConfig)
	return info
}
func (module MemInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	split := strings.Split("gnome-system-monitor -r", " ")
	return nil, exec.Command(split[0], split[1:]...).Start()
}

func (module MemInfo) memInfo() (float64, float64) {
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
