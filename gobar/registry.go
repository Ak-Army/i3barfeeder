package gobar

var moduleRegistry = make(map[string]func() ModuleInterface)

func AddModule(name string, module func() ModuleInterface) {
	moduleRegistry[name] = module
}

// Modules returns the registered constructors keyed by module name. Handy for
// tooling and tests that want to inspect every module without knowing them.
func Modules() map[string]func() ModuleInterface {
	all := make(map[string]func() ModuleInterface, len(moduleRegistry))
	for name, module := range moduleRegistry {
		all[name] = module
	}
	return all
}
