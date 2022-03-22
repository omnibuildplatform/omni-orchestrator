package store

var supportedPlugins = map[string]Plugin{}

func RegisterPlugin(pluginName string, plugin Plugin) {
	if _, ok := supportedPlugins[pluginName]; ok {
		panic("plugin " + pluginName + " already registered")
	}
	supportedPlugins[pluginName] = plugin
}

func RegisterPluginIfNotExists(pluginName string, plugin Plugin) {
	if _, ok := supportedPlugins[pluginName]; !ok {
		supportedPlugins[pluginName] = plugin
	}
}

func PluginRegistered(pluginName string) bool {
	_, ok := supportedPlugins[pluginName]
	return ok
}

func GetRegisteredPluginNames() []string {
	var plugins []string
	for k := range supportedPlugins {
		plugins = append(plugins, k)
	}
	return plugins
}
