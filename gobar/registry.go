package gobar

var moduleRegistry = make(map[string]func() ModuleInterface)

func AddModule(name string, module func() ModuleInterface) {
	moduleRegistry[name] = module
}
