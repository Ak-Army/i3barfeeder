package modules

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("ExternalCmd", func() gobar.ModuleInterface {
		return &ExternalCmd{}
	})
}

type ExternalCmd struct {
	gobar.ModuleInterface
	Command string `json:"command"`
	OnClick string `json:"on_lick"`
	onClick *exec.Cmd
}

func (m *ExternalCmd) InitModule(config json.RawMessage, log xlog.Logger) error {
	if err := json.Unmarshal(config, m); err != nil {
		return err
	}
	if m.OnClick != "" {
		split := strings.Split(m.OnClick, " ")
		m.onClick = exec.Command(split[0], split[1:]...)
	}
	return nil
}

func (m ExternalCmd) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	if m.Command != "" {
		out, err := exec.Command("sh", "-c", m.Command).Output()
		if err != nil {
			info.FullText = err.Error()
			info.TextColor = "#FF2222"
		} else {
			info.FullText = strings.TrimSpace(string(out))
		}
	}
	return info
}

func (m ExternalCmd) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	if cm.Button == 3 && m.onClick != nil {
		return nil, m.onClick.Start()
	}
	return nil, nil
}
