package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("VolumeInfo", func() gobar.ModuleInterface {
		return &VolumeInfo{
			Mixer: "default",
			Step: 1,
			barConfig: defaultBarConfig(),
		}
	})
}

type VolumeInfo struct {
	gobar.ModuleInterface
	Mixer     string `json:"mixer"`
	SControl  string `json:"sControl"`
	Step      int `json:"step"`
	barConfig barConfig
	regex     *regexp.Regexp
}

func (m *VolumeInfo) InitModule(config json.RawMessage, log xlog.Logger) error {
	if config != nil {
		if err := json.Unmarshal(config, m); err != nil {
			return err
		}
		if err := json.Unmarshal(config, &m.barConfig); err != nil {
			return err
		}
	}

	if m.SControl == "" {
		sControl, err := exec.Command("sh", "-c", "amixer -D "+m.Mixer+" scontrols").Output()
		if err == nil {
			regex, _ := regexp.Compile(`'(\w+)',0`)
			m.SControl = regex.FindStringSubmatch(string(sControl))[1]
		} else {
			return fmt.Errorf("unable to find scontrol for mixer: %s, error: %s", m.Mixer, err)
		}
	}
	regex, err := regexp.Compile(`(\d+) \[(\d+)%\].*\[(\w+)\]`)
	if err != nil {
		return fmt.Errorf("regex error: %s", err)
	}
	m.regex = regex

	return nil
}

func (m VolumeInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	out, err := exec.Command("sh", "-c", "amixer -D "+m.Mixer+" get "+m.SControl).Output()
	if err == nil {
		currentVolume := m.volumeInfo(string(out))
		info.ShortText = fmt.Sprintf("%d%s", int(currentVolume), "%")
		info.FullText = makeBar(float64(currentVolume), m.barConfig)
	}

	if err != nil {
		info.FullText = err.Error()
		info.TextColor = "#FF2222"
	}

	return info
}

// {"name":"VolumeInfo","instance":"id_1","button":5,"x":2991,"y":12}
func (m VolumeInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	var cmd string
	switch cm.Button {
	case 3: // right click, mute/unmute
		cmd = fmt.Sprintf("amixer -D %s sset %s toggle", m.Mixer, m.SControl)
	case 4: // scroll up, increase
		cmd = fmt.Sprintf("amixer -D %s sset %s %d%%+ unmute", m.Mixer, m.SControl, m.Step)
	case 5: // scroll down, decrease
		cmd = fmt.Sprintf("amixer -D %s sset %s %d%%- unmute", m.Mixer, m.SControl, m.Step)
	}
	if cmd != "" {
		out, err := exec.Command("sh", "-c", cmd).Output()
		if err == nil {
			currentVolume := m.volumeInfo(string(out))
			info.ShortText = fmt.Sprintf("%d%s", int(currentVolume), "%")
			info.FullText = makeBar(float64(currentVolume), m.barConfig)
		}
	}
	return &info, nil
}

func (m VolumeInfo) volumeInfo(out string) float64 {
	volumes := m.regex.FindStringSubmatch(out)
	if len(volumes) == 0 || volumes[3] == "off" {
		return float64(0)
	} else {
		currentVolume, err := strconv.ParseFloat(m.regex.FindStringSubmatch(string(out))[2], 64)
		if err == nil {
			return float64(currentVolume)
		}
	}
	return float64(0)
}
