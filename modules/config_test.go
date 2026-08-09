package modules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

// configKeys collects every key a module accepts, walking into nested structs so
// the shared bar settings (loaded separately into the private barConfig field)
// count too.
func configKeys(v reflect.Value, into map[string]bool) {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if tag := field.Tag.Get("config"); tag != "" {
			into[strings.Split(tag, ",")[0]] = true
		}
		switch field.Type.Kind() {
		case reflect.Struct:
			configKeys(v.Field(i), into)
		case reflect.Ptr:
			if field.Type.Elem().Kind() == reflect.Struct {
				configKeys(v.Field(i), into)
			}
		}
	}
}

func moduleConfigKeys(t *testing.T) map[string]map[string]bool {
	t.Helper()
	keys := make(map[string]map[string]bool)
	for name, newModule := range gobar.Modules() {
		set := make(map[string]bool)
		configKeys(reflect.ValueOf(newModule()), set)
		keys[name] = set
	}
	return keys
}

type barConfigFile struct {
	Blocks []struct {
		Module string                     `json:"module"`
		Config map[string]json.RawMessage `json:"config"`
	} `json:"blocks"`
}

// TestConfigFilesOnlyUseKnownKeys guards the failure mode that costs the most
// time to spot: the loader matches keys against `config:"..."` tags exactly and
// drops anything else without a word, so a typo — or a `json:"..."` tag left on
// a field — shows up as a block stuck on its built-in default rather than as an
// error.
func TestConfigFilesOnlyUseKnownKeys(t *testing.T) {
	keys := moduleConfigKeys(t)

	files, err := filepath.Glob("../*.json*")
	if err != nil {
		t.Fatalf("glob: %s", err)
	}
	if len(files) == 0 {
		t.Skip("no config files to check")
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read: %s", err)
			}
			var conf barConfigFile
			if err := json.Unmarshal(raw, &conf); err != nil {
				t.Fatalf("not valid JSON: %s", err)
			}
			for i, block := range conf.Blocks {
				known, ok := keys[block.Module]
				if !ok {
					t.Errorf("block %d: unknown module %q", i, block.Module)
					continue
				}
				for key := range block.Config {
					if !known[key] {
						t.Errorf("block %d (%s): key %q is not accepted by the module, it will be ignored",
							i, block.Module, key)
					}
				}
			}
		})
	}
}

// TestModulesDeclareConfigTags catches a field that is meant to be configurable
// but carries no `config:"..."` tag — it would silently never be filled in.
func TestModulesDeclareConfigTags(t *testing.T) {
	for name, newModule := range gobar.Modules() {
		v := reflect.ValueOf(newModule())
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		typ := v.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" || field.Anonymous {
				continue // unexported or the embedded BaseModule
			}
			if field.Tag.Get("config") == "" {
				t.Errorf("%s.%s is exported but has no `config:\"...\"` tag, it can never be configured",
					name, field.Name)
			}
			if field.Tag.Get("json") != "" {
				t.Errorf("%s.%s carries a `json:\"...\"` tag, which the config loader ignores",
					name, field.Name)
			}
		}
	}
}
