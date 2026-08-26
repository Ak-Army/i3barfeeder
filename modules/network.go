package modules

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Ak-Army/config"
	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
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
	gobar.BaseModule
	InterfaceName []string `config:"interfaceName"`
	barConfig     barConfig
	currRx        uint64
	currTx        uint64
	log           xlog.Logger
}

func (m *Network) InitModule(c *config.SubConfig, log xlog.Logger) error {
	m.log = log
	if c != nil {
		if err := c.Load(m); err != nil {
			return err
		}
		if err := c.Load(&m.barConfig); err != nil {
			return err
		}
	}
	m.collectData()

	return nil
}

func (m *Network) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	name := m.collectData()
	text := fmt.Sprintf("%s %s / %s", name,
		byteSize(delta(m.currRx, m.currRx)),
		byteSize(delta(m.currTx, m.currTx)))
	info.ShortText = text
	info.FullText = text

	return info
}

func (m *Network) collectData() string {
	// Reference: man 5 proc, Documentation/filesystems/proc.txt in Linux source code
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		m.log.Warn("File open error", err)
		m.currTx = 0
		m.currRx = 0
		return "none"
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
		m.currRx = rxBytes
		m.currTx = txBytes

		return m.wirelessName(name)
	}
	if err := scanner.Err(); err != nil {
		m.log.Warn("File scan error", err)
	}
	m.currTx = 0
	m.currRx = 0
	return "none"
}

func (m *Network) wirelessName(name string) string {
	out, err := exec.Command("iwconfig", name).Output()
	if err != nil {
		return name
	}
	return parseWireless(string(out), name)
}

// parseWireless pulls the SSID and the signal level out of iwconfig's output,
// falling back to the interface name when the driver reports neither.
func parseWireless(out string, name string) string {
	ssids := strings.SplitN(out, "ESSID:\"", 2)
	if len(ssids) < 2 {
		return name
	}
	ssid := strings.Split(ssids[1], "\"")[0]
	if ssid == "" {
		return name
	}

	// Not every driver reports a signal level, so the name stays SSID-only there.
	levels := strings.SplitN(out, "Signal level=", 2)
	if len(levels) < 2 {
		return ssid
	}
	sigLevel := strings.Split(levels[1], " ")[0]
	return fmt.Sprintf("%s (%s dB)", ssid, sigLevel)
}

func (m *Network) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	return nil, nil
}
