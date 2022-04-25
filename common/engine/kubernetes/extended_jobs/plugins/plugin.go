package plugins

var SupportedJobPlugins = map[string]JobPlugin{}

func RegisterPlugin(pluginName string, plugin JobPlugin) {
	if _, ok := SupportedJobPlugins[pluginName]; ok {
		panic("plugin " + pluginName + " already registered")
	}
	SupportedJobPlugins[pluginName] = plugin
}

func GetRegisteredJobPluginNames() []string {
	var plugins []string
	for k := range SupportedJobPlugins {
		plugins = append(plugins, k)
	}
	return plugins
}
