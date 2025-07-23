package modules

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Ak-Army/i3barfeeder/gobar"

	"github.com/Ak-Army/xlog"
)

func init() {
	gobar.AddModule("Network", func() gobar.ModuleInterface {
		return &Network{
			InterfaceName: []string{"tun1"},
			barConfig:     defaultBarConfig(),
		}
	})
}

type Network struct {
	gobar.ModuleInterface
	InterfaceName []string `json:"InterfaceName"`
	barConfig     barConfig
	currRx        uint64
	currTx        uint64
	log           xlog.Logger
}

func (m *Network) InitModule(config json.RawMessage, log xlog.Logger) error {
	m.log = log
	if config != nil {
		if err := json.Unmarshal(config, m); err != nil {
			return err
		}
		if err := json.Unmarshal(config, &m.barConfig); err != nil {
			return err
		}
	}
	_, m.currRx, m.currTx = m.collectData()

	return nil
}

func (m *Network) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	name, currRx, currTx := m.collectData()
	info.ShortText = fmt.Sprintf("%s %s / %s", name, byteSize(currRx-m.currRx), byteSize(currTx-m.currTx))
	info.FullText = fmt.Sprintf("%s %s / %s", name, byteSize(currRx-m.currRx), byteSize(currTx-m.currTx))
	m.currRx, m.currTx = currRx, currTx
	return info
}

func (m *Network) collectData() (string, uint64, uint64) {
	// Reference: man 5 proc, Documentation/filesystems/proc.txt in Linux source code
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		m.log.Warn("File open error", err)
		return "none", 0, 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Reference: dev_seq_printf_stats in Linux source code
		kv := strings.SplitN(scanner.Text(), ":", 2)
		if len(kv) != 2 {
			continue
		}
		fields := strings.Fields(kv[1])
		if len(fields) < 16 {
			continue
		}
		name := strings.TrimSpace(kv[0])
		found := false
		for _, n := range m.InterfaceName {
			if name == n {
				found = true
			}
		}
		if !found {
			continue
		}
		rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			m.log.Warnf("Unable to parse RX field: %s", fields[0])
		}
		txBytes, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			m.log.Warnf("Unable to parse TX field: %s", fields[8])
		}
		out, err := exec.Command("iwconfig", name).Output()
		if err == nil {
			ssids := strings.SplitN(string(out), "ESSID:\"", 2)
			if len(ssids) < 2 {
				return name, rxBytes, txBytes
			}
			ssid := ssids[1]
			ssid = strings.Split(ssid, "\"")[0]
			sigLevel := strings.SplitN(string(out), "Signal level=", 2)[1]
			sigLevel = strings.Split(sigLevel, " ")[0]
			name = fmt.Sprintf("%s (%s dB)", ssid, sigLevel)
		}
		return name, rxBytes, txBytes
	}
	if err := scanner.Err(); err != nil {
		m.log.Warn("File scan error", err)
		return "none", 0, 0
	}
	return "none", 0, 0
}

func (m Network) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
