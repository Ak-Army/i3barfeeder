package modules

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
)

// openURL hands the URL to the first browser installed. It deliberately does not
// wait for the process: there is a single click goroutine for the whole bar, and
// a browser started without a running instance only exits when the user closes
// it, which would leave every block unclickable until then.
func openURL(url string) error {
	try := []string{"xdg-open", "brave-browser", "google-chrome", "firefox", "open"}
	for _, bin := range try {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		if err := startDetached(exec.Command(bin, url)); err != nil {
			continue
		}
		return nil
	}
	return fmt.Errorf("unable to open URL in a browser: none of %v is installed", try)
}

// startDetached runs cmd without blocking, reaping it in the background so it
// does not linger as a zombie.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// wrapIndex moves i by step within [0, n), wrapping at whichever end it runs
// off. An index left over from a longer list wraps too: the ticket list shrinks
// whenever a project stops resolving, and indexing with a stale one used to
// panic and take the process down.
func wrapIndex(i, step, n int) int {
	if n <= 0 {
		return 0
	}
	i += step
	if i < 0 || i >= n {
		if step < 0 {
			return n - 1
		}
		return 0
	}
	return i
}

type barConfig struct {
	BarSize  int    `config:"barSize"`
	BarFull  string `config:"barFull"`
	BarEmpty string `config:"barEmpty"`
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
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "Error reading %s: %s\n", fileName, err)
			}
			return
		}
		if !callback(string(line)) {
			return
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

// delta guards against the counters going backwards, which happens whenever the
// interface disappears and collectData falls back to zero. An unchecked uint64
// subtraction would render as exabytes.
func delta(curr, prev uint64) uint64 {
	if curr < prev {
		return 0
	}
	return curr - prev
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
