package modules

import (
	"os/exec"
	"strings"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

type ExternalCmd struct {
	gobar.ModuleInterface
	command string
	onClick *exec.Cmd
}

func (module *ExternalCmd) InitModule(config gobar.Config) error {
	if command, ok := config["command"].(string); ok {
		module.command = command
	}

	if command, ok := config["on_click"].(string); ok {
		split := strings.Split(command, " ")
		module.onClick = exec.Command(split[0], split[1:]...)
	}
	return nil
}

func (module ExternalCmd) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	if module.command != "" {
		out, err := exec.Command("sh", "-c", module.command).Output()
		if err != nil {
			info.FullText = err.Error()
			info.TextColor = "#FF2222"
		} else {
			info.FullText = strings.TrimSpace(string(out))
		}
	}
	return info
}

func (module ExternalCmd) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	if module.onClick != nil {
		return nil, module.onClick.Start()
	}
	return nil, nil
}

