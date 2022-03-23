package plugin

var SupportedPlugins = map[string]Plugin{}

func RegisterPlugin(pluginName string, plugin Plugin) {
	if _, ok := SupportedPlugins[pluginName]; ok {
		panic("plugin " + pluginName + " already registered")
	}
	SupportedPlugins[pluginName] = plugin
}

func RegisterPluginIfNotExists(pluginName string, plugin Plugin) {
	if _, ok := SupportedPlugins[pluginName]; !ok {
		SupportedPlugins[pluginName] = plugin
	}
}

func PluginRegistered(pluginName string) bool {
	_, ok := SupportedPlugins[pluginName]
	return ok
}

func GetRegisteredPluginNames() []string {
	var plugins []string
	for k := range SupportedPlugins {
		plugins = append(plugins, k)
	}
	return plugins
}
