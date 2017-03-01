package modules

import (
	"syscall"
	"fmt"
	"reflect"

	"github.com/Ak-Army/i3barfeeder/gobar"
	"strings"
	"os/exec"
)

type DiskUsage struct {
	gobar.ModuleInterface
	path      string
	barConfig barConfig
}

func (module *DiskUsage) InitModule(config gobar.Config) error {
	module.path = keyExists(config, "path", reflect.String, "/").(string)
	module.barConfig.barSize = keyExists(config, "barSize", reflect.Int, 10).(int)
	module.barConfig.barFull = keyExists(config, "barFull", reflect.String, "■").(string)
	module.barConfig.barEmpty = keyExists(config, "barEmpty", reflect.String, "□").(string)

	return nil
}

func (module DiskUsage) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	free, total := module.diskUsage()
	freePercent := 100 * (free / total)
	info.ShortText = fmt.Sprintf("%d %s", int(freePercent), "%")
	info.FullText = makeBar(freePercent, module.barConfig)
	return info
}
func (module DiskUsage) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	split := strings.Split("gnome-system-monitor -f", " ")
	return nil, exec.Command(split[0], split[1:]...).Start()
}

func (module DiskUsage) diskUsage() (free float64, total float64) {
	// Return bytes free and total bytes.
	buf := new(syscall.Statfs_t)
	syscall.Statfs(module.path, buf)
	free = float64(buf.Bsize) * float64(buf.Bfree)
	total = float64(buf.Bsize) * float64(buf.Blocks)
	return free, total
}