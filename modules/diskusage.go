package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("DiskUsage", func() gobar.ModuleInterface {
		return &DiskUsage{
			Path:      "/",
			barConfig: defaultBarConfig(),
		}
	})
}

type DiskUsage struct {
	gobar.ModuleInterface
	Path      string
	barConfig barConfig
}

func (m *DiskUsage) InitModule(config json.RawMessage, log xlog.Logger) error {
	if config != nil {
		if err := json.Unmarshal(config, m); err != nil {
			return err
		}
		return json.Unmarshal(config, &m.barConfig)
	}
	return nil
}

func (m DiskUsage) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	free, total := m.diskUsage()
	freePercent := 100 - (100 * (free / total))
	info.ShortText = fmt.Sprintf("%d %s", int(freePercent), "%")
	info.FullText = makeBar(freePercent, m.barConfig)
	return info
}

func (m DiskUsage) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	split := strings.Split("gnome-system-monitor -f", " ")
	return nil, exec.Command(split[0], split[1:]...).Start()
}

func (m DiskUsage) diskUsage() (free float64, total float64) {
	// Return bytes free and total bytes.
	buf := new(syscall.Statfs_t)
	err := syscall.Statfs(m.Path, buf)
	if err != nil {
		return 0, 0
	}
	free = float64(buf.Bsize) * float64(buf.Bfree)
	total = float64(buf.Bsize) * float64(buf.Blocks)
	return free, total
}
