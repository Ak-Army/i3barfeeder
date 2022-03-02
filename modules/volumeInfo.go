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
			Step:      1,
			barConfig: defaultBarConfig(),
		}
	})
}

type VolumeInfo struct {
	gobar.ModuleInterface
	Step      int `json:"step"`
	barConfig barConfig
	regex     *regexp.Regexp
	regexCard *regexp.Regexp
	log       xlog.Logger
	card      string
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
	m.log = log
	regex, err := regexp.Compile(`(?m): (.*)\n.*front-left: \d+ /[ ]+(\d+)% / [^ ]+ dB`)

	if err != nil {
		return fmt.Errorf("regex error: %s", err)
	}
	regexCard, err := regexp.Compile(`\.card = "(\d+)"`)
	if err != nil {
		return fmt.Errorf("regex error: %s", err)
	}
	m.regex = regex
	m.regexCard = regexCard
	m.card = "0"

	return nil
}

func (m *VolumeInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	out, err := exec.Command("sh", "-c", "pactl list sinks").Output()
	if err == nil {
		currentVolume := m.volumeInfo(string(out))
		info.ShortText = fmt.Sprintf("%f%s", currentVolume, "%")
		m.log.Debug("currentVolume:", currentVolume)
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
		cmd = `pactl set-sink-mute ` + m.card + ` toggle`
	case 4: // scroll up, increase
		cmd = `pactl set-sink-mute ` + m.card + ` false; pactl set-sink-volume ` + m.card + ` +5%`
	case 5: // scroll down, decrease
		cmd = `pactl set-sink-mute ` + m.card + ` false; pactl set-sink-volume ` + m.card + ` -5%`
	}
	m.log.Info(cmd)
	if cmd != "" {
		_, err := exec.Command("sh", "-c", cmd).Output()
		if err == nil {
			m.UpdateInfo(info)
		}
	}
	return &info, nil
}

func (m *VolumeInfo) volumeInfo(out string) float64 {
	volumes := m.regex.FindStringSubmatch(out)
	card := m.regexCard.FindStringSubmatch(out)
	currentVolume := float64(0)

	if len(card) == 2 {
		m.card = card[1]
	}
	m.log.Debug("card:", card)

	if len(volumes) == 0 || volumes[1] == "off" || volumes[1] == "igen" {
		return currentVolume
	}
	var err error
	m.log.Debug("currentVolume222222:", volumes[2])
	currentVolume, err = strconv.ParseFloat(volumes[2], 64)
	if err == nil {
		return currentVolume
	}
	return float64(0)
}
