package modules

import (
	"fmt"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

type VolumeInfo struct {
	gobar.ModuleInterface
	path      string
	barConfig barConfig
	mixer     string
	sControl  string
	step      int
	regex     *regexp.Regexp
}

func (module *VolumeInfo) InitModule(config gobar.Config) error {
	module.path = keyExists(config, "path", reflect.String, "/").(string)

	module.barConfig.barSize = keyExists(config, "barSize", reflect.Int, 10).(int)
	module.barConfig.barFull = keyExists(config, "barFull", reflect.String, "■").(string)
	module.barConfig.barEmpty = keyExists(config, "barEmpty", reflect.String, "□").(string)

	module.mixer = keyExists(config, "mixer", reflect.String, "default").(string)
	module.step = keyExists(config, "step", reflect.Int, 1).(int)
	if sControl, ok := config["scontrol"].(string); ok {
		module.sControl = sControl
	} else {
		sControl, err := exec.Command("sh", "-c", "amixer -D default scontrols").Output()
		if err == nil {
			regex, _ := regexp.Compile(`'(\w+)',0`)
			module.sControl = regex.FindStringSubmatch(string(sControl))[1]
		} else {
			return fmt.Errorf("Cant find scontrol for mixer: %s, error: %s", module.mixer, err)
		}
	}
	regex, err := regexp.Compile(`(\d+) \[(\d+)%\].*\[(\w+)\]`)
	if err != nil {
		return fmt.Errorf("Regex error: %s", err)
	}
	module.regex = regex

	return nil
}

func (module VolumeInfo) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	out, err := exec.Command("sh", "-c", "amixer -D "+module.mixer+" get "+module.sControl).Output()
	if err == nil {
		currentVolume := module.volumeInfo(string(out))
		info.ShortText = fmt.Sprintf("%d%s", int(currentVolume), "%")
		info.FullText = makeBar(float64(currentVolume), module.barConfig)
	}

	if err != nil {
		info.FullText = err.Error()
		info.TextColor = "#FF2222"
	}

	return info
}

//{"name":"VolumeInfo","instance":"id_1","button":5,"x":2991,"y":12}
func (module VolumeInfo) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	var cmd string
	switch cm.Button {
	case 3: //right click, mute/unmute
		cmd = fmt.Sprintf("amixer -D %s sset %s toggle", module.mixer, module.sControl)
	case 4: //scroll up, increase
		cmd = fmt.Sprintf("amixer -D %s sset %s %d%%+ unmute", module.mixer, module.sControl, module.step)
	case 5: //scroll down, decrease
		cmd = fmt.Sprintf("amixer -D %s sset %s %d%%- unmute", module.mixer, module.sControl, module.step)
	}
	if cmd != "" {
		out, err := exec.Command("sh", "-c", cmd).Output()
		if err == nil {
			currentVolume := module.volumeInfo(string(out))
			info.ShortText = fmt.Sprintf("%d%s", int(currentVolume), "%")
			info.FullText = makeBar(float64(currentVolume), module.barConfig)
		}
	}
	return &info, nil
}

func (module VolumeInfo) volumeInfo(out string) float64 {
	volumes := module.regex.FindStringSubmatch(out)
	if len(volumes) == 0 || volumes[3] == "off" {
		return float64(0)
	} else {
		currentVolume, err := strconv.ParseFloat(module.regex.FindStringSubmatch(string(out))[2], 64)
		if err == nil {
			return float64(currentVolume)
		}
	}
	return float64(0)
}
