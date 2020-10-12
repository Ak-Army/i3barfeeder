package modules

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
)

type barConfig struct {
	BarSize  int    `json:"barSize"`
	BarFull  string `json:"barFull"`
	BarEmpty string `json:"barEmpty"`
}

func defaultBarConfig() barConfig {
	return barConfig{
		BarSize:  10,
		BarFull:  "■",
		BarEmpty: "□",
	}
}

func makeBar(freePercent float64, barConfig barConfig) string {
	var bar bytes.Buffer
	cutoff := int(freePercent * .01 * float64(barConfig.BarSize))
	for i := 0; i < barConfig.BarSize; i += 1 {
		if i < cutoff {
			bar.WriteString(barConfig.BarFull)
		} else {
			bar.WriteString(barConfig.BarEmpty)
		}
	}
	return bar.String()
}

func readLines(fileName string, callback func(string) bool) {
	fin, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The file %s does not exist!\n", fileName)
		return
	}
	defer fin.Close()

	reader := bufio.NewReader(fin)
	for line, _, err := reader.ReadLine(); err != io.EOF; line, _, err = reader.ReadLine() {
		if !callback(string(line)) {
			break
		}
	}
}

func byteSize(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

type sortedMap struct {
	m map[string]int64
	s []string
}

func (sm *sortedMap) Len() int {
	return len(sm.m)
}

func (sm *sortedMap) Less(i, j int) bool {
	a, b := sm.m[sm.s[i]], sm.m[sm.s[j]]
	if a != b {
		// Order by decreasing value.
		return a > b
	} else {
		// Otherwise, alphabetical order.
		return sm.s[j] > sm.s[i]
	}
}

func (sm *sortedMap) Swap(i, j int) {
	sm.s[i], sm.s[j] = sm.s[j], sm.s[i]
}

func sortedKeys(m map[string]int64) []string {
	sm := new(sortedMap)
	sm.m = m
	sm.s = make([]string, len(m))
	i := 0
	for key, _ := range m {
		sm.s[i] = key
		i++
	}
	sort.Sort(sm)
	return sm.s
}
