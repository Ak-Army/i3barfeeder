package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/Ak-Army/config"
	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("VolumeInfo", func() gobar.ModuleInterface {
		return &VolumeInfo{
			// 5% was the hard-coded step before it became configurable; keep it
			// as the default so an existing config behaves the same.
			Step:      5,
			barConfig: defaultBarConfig(),
		}
	})
}

// defaultSink is what pactl calls the sink the desktop is currently using.
// Addressing it by name beats guessing an index: sink numbering is not stable
// (a running PipeWire hands out ids like 56), and `0` only ever worked because
// pactl happens to fall back to the default for it.
const defaultSink = "@DEFAULT_SINK@"

type VolumeInfo struct {
	gobar.BaseModule
	Step      int `config:"step"`
	barConfig barConfig
	regex     *regexp.Regexp
	log       xlog.Logger
}

func (m *VolumeInfo) InitModule(c *config.SubConfig, log xlog.Logger) error {
	if c != nil {
		if err := c.Load(m); err != nil {
			return err
		}
		if err := c.Load(&m.barConfig); err != nil {
			return err
		}
	}
	m.log = log
	if m.Step <= 0 {
		m.Step = 5
	}
	regex, err := regexp.Compile(`(?m): (.*)\n.*front-left: \d+ /[ ]+(\d+)% / [^ ]+ dB`)

	if err != nil {
		return fmt.Errorf("regex error: %s", err)
	}
	m.regex = regex

	return nil
}

func (m *VolumeInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	// LC_ALL=C keeps pactl's output (and thus the regexes below) language
	// independent, whatever locale i3 was started with.
	cmd := exec.Command("sh", "-c", "pactl list sinks")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err == nil {
		currentVolume := m.volumeInfo(string(out))
		info.ShortText = fmt.Sprintf("%.0f%s", currentVolume, "%")
		if currentVolume >= 100 {
			currentVolume -= 99
			info.TextColor = "#FF2222"
		}
		info.FullText = makeBar(currentVolume, m.barConfig)
	}

	if err != nil {
		info.FullText = err.Error()
		info.TextColor = "#FF2222"
	}

	return info
}

// {"name":"VolumeInfo","instance":"id_1","button":5,"x":2991,"y":12}
func (m *VolumeInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	var cmd string
	switch cm.Button {
	case 3: // right click, mute/unmute
		cmd = "pactl set-sink-mute " + defaultSink + " toggle"
	case 4: // scroll up, increase
		cmd = m.volumeCmd("+")
	case 5: // scroll down, decrease
		cmd = m.volumeCmd("-")
	}
	m.log.Info(cmd)
	if cmd != "" {
		_, err := exec.Command("sh", "-c", cmd).Output()
		if err == nil {
			info = m.UpdateInfo(info)
		}
	}
	return &info, nil
}

// volumeCmd unmutes and moves the volume by the configured step in the given
// direction ("+" or "-").
func (m *VolumeInfo) volumeCmd(sign string) string {
	return fmt.Sprintf("pactl set-sink-mute %s false; pactl set-sink-volume %s %s%d%%",
		defaultSink, defaultSink, sign, m.Step)
}

func (m *VolumeInfo) volumeInfo(out string) float64 {
	volumes := m.regex.FindStringSubmatch(out)
	currentVolume := float64(0)

	if len(volumes) == 0 || volumes[1] == "off" || volumes[1] == "yes" {
		return currentVolume
	}
	var err error
	currentVolume, err = strconv.ParseFloat(volumes[2], 64)
	if err == nil {
		return currentVolume
	}
	return float64(0)
}
