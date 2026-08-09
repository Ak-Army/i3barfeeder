package modules

import (
	"os/exec"
	"strings"

	"github.com/Ak-Army/config"
	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("ExternalCmd", func() gobar.ModuleInterface {
		return &ExternalCmd{}
	})
}

type ExternalCmd struct {
	gobar.BaseModule
	//Command to be executed (using "/bin/sh -c [command]")
	Exec string `config:"exec"`

	// Conditional command that, if defined, needs to exit successfully
	// before the main exec command is invoked.
	// Default: ""
	ExecIf string `config:"exec_if"`

	//"click-(left|middle|right)" will be executed using "/bin/sh -c [command]"
	ClickLeft   string `config:"click_left"`
	ClickMiddle string `config:"click_middle"`
	ClickRight  string `config:"click_right"`

	// "scroll-(up|down)" will be executed using "/bin/sh -c [command]"
	ScrollUp   string `config:"scroll_up"`
	ScrollDown string `config:"scroll_down"`
}

func (m *ExternalCmd) InitModule(c *config.SubConfig, log xlog.Logger) error {
	if c != nil {
		if err := c.Load(m); err != nil {
			return err
		}
	}
	return nil
}

func (m *ExternalCmd) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	if m.ExecIf != "" {
		_, err := exec.Command("sh", "-c", m.ExecIf).Output()
		if err != nil {
			return info
		}
	}
	m.execCommand(m.Exec, &info)

	return info
}

func (m *ExternalCmd) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	switch cm.Button {
	case 1: // left button
		if m.ClickLeft != "" {
			m.execCommand(m.ClickLeft, &info)
			return &info, nil
		}
	case 2: // middle button
		if m.ClickMiddle != "" {
			m.execCommand(m.ClickMiddle, &info)
			return &info, nil
		}
	case 3: // right click, join zoom
		if m.ClickRight != "" {
			m.execCommand(m.ClickRight, &info)
			return &info, nil
		}
	case 4: // scroll up, decrease
		if m.ScrollUp != "" {
			m.execCommand(m.ScrollUp, &info)
			return &info, nil
		}
	case 5: // scroll down, decrease
		if m.ScrollDown != "" {
			m.execCommand(m.ScrollDown, &info)
			return &info, nil
		}
	}
	return nil, nil
}

func (m *ExternalCmd) execCommand(cmd string, info *gobar.BlockInfo) {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		info.ShortText = err.Error()
		info.FullText = err.Error()
		return
	}
	text := strings.TrimRight(string(out), "\n")
	info.ShortText = text
	info.FullText = text
	return
}
