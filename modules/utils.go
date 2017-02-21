package modules

import (
	"reflect"
	"bytes"
	"os"
	"fmt"
	"bufio"
	"io"
)

type barConfig struct {
	barSize int
	barFull string
	barEmpty string
}

func keyExists(config map[string]interface{}, key string, keyType reflect.Kind, defaultValue interface{}) interface{} {
	if value, ok := config[key]; ok && reflect.TypeOf(config[key]).Kind() == keyType {
		return value
	}
	return defaultValue
}

func makeBar(freePercent float64, barConfig barConfig) string {
	var bar bytes.Buffer
	cutoff := int(freePercent * .01 * float64(barConfig.barSize))
	for i := 0; i < barConfig.barSize; i += 1 {
		if i < cutoff {
			bar.WriteString(barConfig.barFull)
		} else {
			bar.WriteString(barConfig.barEmpty)
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

