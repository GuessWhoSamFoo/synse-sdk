package sdk

import (
	"fmt"

	"runtime"

	"github.com/vapor-ware/synse-sdk/sdk/config"
)

// Plugin represents an instance of a Synse plugin. Along with metadata
// and definable handlers, it contains a gRPC server to handle the plugin
// requests.
type Plugin struct {
	Config   *config.PluginConfig
	server   *Server
	handlers *Handlers
	dm       *DataManager
	v        *VersionInfo
}

// NewPlugin creates a new Plugin instance
func NewPlugin(handlers *Handlers) *Plugin {
	p := &Plugin{}
	p.handlers = handlers
	p.v = emptyVersionInfo()
	return p
}

// SetVersion sets the VersionInfo for the Plugin.
func (p *Plugin) SetVersion(info VersionInfo) {
	p.v.Merge(&info)
}

// SetConfig manually sets the configuration of the Plugin.
func (p *Plugin) SetConfig(config *config.PluginConfig) error {
	err := config.Validate()
	if err != nil {
		return err
	}
	p.Config = config

	SetLogLevel(p.Config.Debug)
	return nil
}

// Configure reads in the config file and uses it to set the Plugin configuration.
func (p *Plugin) Configure() error {
	cfg, err := config.NewPluginConfig()
	if err != nil {
		return err
	}
	p.Config = cfg
	SetLogLevel(p.Config.Debug)
	return nil
}

// RegisterDevices registers all of the configured devices (via their proto and
// instance config) with the plugin.
func (p *Plugin) RegisterDevices() error {
	return registerDevicesFromConfig(p.handlers.Device, p.Config.AutoEnumerate)
}

// Run starts the Plugin server which begins listening for gRPC requests.
func (p *Plugin) Run() error {
	err := p.setup()
	if err != nil {
		return err
	}
	p.logInfo()

	// Start the go routines to poll devices and to update internal state
	// with those readings.
	p.dm.goPollData()
	p.dm.goUpdateData()

	// Start the gRPC server
	return p.server.serve()
}

// setup is the pre-run stage where the Plugin handlers and configuration
// are validated and runtime components of the plugin are initialized.
func (p *Plugin) setup() error {
	// validate that handlers are set
	err := validateHandlers(p.handlers)
	if err != nil {
		return err
	}

	// validate that configuration is set
	if p.Config == nil {
		return fmt.Errorf("plugin must be configured before it is run")
	}

	// Setup the transaction cache
	SetupTransactionCache(p.Config.Settings.Transaction.TTL)

	// Register a new Server and DataManager for the Plugin. This should
	// be done prior to running the plugin, as opposed to on initialization
	// of the Plugin struct, because their configuration is configuration
	// dependent. The Plugin should be configured prior to running.
	p.server = NewServer(p)
	p.dm = NewDataManager(p)

	return nil
}

// logInfo logs out the information about the plugin. This is called just before the
// plugin begins running all of its components.
func (p *Plugin) logInfo() {
	Logger.Info("Plugin Info:")
	Logger.Infof(" Name:        %s", p.Config.Name)
	Logger.Infof(" Version:     %s", p.v.VersionString)
	Logger.Infof(" SDK Version: %s", SDKVersion)
	Logger.Infof(" Git Commit:  %s", p.v.GitCommit)
	Logger.Infof(" Git Tag:     %s", p.v.GitTag)
	Logger.Infof(" Go Version:  %s", p.v.GoVersion)
	Logger.Infof(" Build Date:  %s", p.v.BuildDate)
	Logger.Infof(" OS:          %s", runtime.GOOS)
	Logger.Infof(" Arch:        %s", runtime.GOARCH)
	Logger.Debug("Plugin Config:")
	Logger.Debugf(" %#v", p.Config)
	Logger.Info("Registered Devices:")
	for id, dev := range deviceMap {
		Logger.Infof(" %v (%v)", id, dev.Model())
	}
	Logger.Info("--------------------------------")
}
