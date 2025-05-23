package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/alessio/shellescape"
	"github.com/nodeset-org/hyperdrive-daemon/shared"
	"github.com/nodeset-org/hyperdrive-daemon/shared/config/ids"
	"github.com/nodeset-org/hyperdrive-daemon/shared/config/migration"
	"github.com/rocket-pool/node-manager-core/config"
	"github.com/rocket-pool/node-manager-core/log"
	"gopkg.in/yaml.v3"
)

// =========================
// === Hyperdrive Config ===
// =========================

const (
	// Tags
	hyperdriveTag string = "nodeset/hyperdrive:v" + shared.HyperdriveVersion
)

// The master configuration struct
type HyperdriveConfig struct {
	// General settings
	Network                  config.Parameter[config.Network]
	ClientMode               config.Parameter[config.ClientMode]
	EnableIPv6               config.Parameter[bool]
	ProjectName              config.Parameter[string]
	ApiPort                  config.Parameter[uint16]
	UserDataPath             config.Parameter[string]
	AutoTxMaxFee             config.Parameter[float64]
	MaxPriorityFee           config.Parameter[float64]
	AutoTxGasThreshold       config.Parameter[float64]
	AdditionalDockerNetworks config.Parameter[string]
	ClientTimeout            config.Parameter[uint16]

	// The Docker Hub tag for the daemon container
	ContainerTag config.Parameter[string]

	// Logging
	Logging *config.LoggerConfig

	// Execution client settings
	LocalExecutionClient    *config.LocalExecutionConfig
	ExternalExecutionClient *config.ExternalExecutionConfig

	// Beacon node settings
	LocalBeaconClient    *config.LocalBeaconConfig
	ExternalBeaconClient *config.ExternalBeaconConfig

	// Fallback clients
	Fallback *config.FallbackConfig

	// Metrics
	Metrics *config.MetricsConfig

	// MEV-Boost
	MevBoost *MevBoostConfig

	// Modules
	Modules map[string]any

	// Internal fields
	Version                 string
	hyperdriveUserDirectory string
	networkSettings         []*HyperdriveSettings
}

// Load configuration settings from a file
func LoadFromFile(path string, networks []*HyperdriveSettings) (*HyperdriveConfig, error) {
	// Return nil if the file doesn't exist
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}

	// Read the file
	configBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read Hyperdrive settings file at %s: %w", shellescape.Quote(path), err)
	}

	// Attempt to parse it out into a settings map
	var settings map[string]any
	if err := yaml.Unmarshal(configBytes, &settings); err != nil {
		return nil, fmt.Errorf("could not parse settings file: %w", err)
	}

	// Deserialize it into a config object
	cfg, err := NewHyperdriveConfig(filepath.Dir(path), networks)
	if err != nil {
		return nil, fmt.Errorf("could not create Hyperdrive config: %w", err)
	}
	err = cfg.Deserialize(settings)
	if err != nil {
		return nil, fmt.Errorf("could not deserialize settings file: %w", err)
	}

	return cfg, nil
}

// Creates a new Hyperdrive configuration instance
func NewHyperdriveConfig(hdDir string, networks []*HyperdriveSettings) (*HyperdriveConfig, error) {
	return NewHyperdriveConfigForNetwork(hdDir, networks, config.Network_Mainnet) // Default to mainnet
}

// Creates a new Hyperdrive configuration instance for a specific network
func NewHyperdriveConfigForNetwork(hdDir string, networks []*HyperdriveSettings, selectedNetwork config.Network) (*HyperdriveConfig, error) {
	cfg := &HyperdriveConfig{
		hyperdriveUserDirectory: hdDir,
		networkSettings:         networks,
		Modules:                 map[string]any{},

		ProjectName: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.ProjectNameID,
				Name:               "Project Name",
				Description:        "This is the prefix that will be attached to all of the Docker containers managed by Hyperdrive.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_BeaconNode, config.ContainerID_Daemon, config.ContainerID_ExecutionClient, config.ContainerID_Exporter, config.ContainerID_Grafana, config.ContainerID_Prometheus, config.ContainerID_ValidatorClient},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]string{
				config.Network_All: "hyperdrive",
			},
		},

		ApiPort: config.Parameter[uint16]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.ApiPortID,
				Name:               "Daemon API Port",
				Description:        "The port that Hyperdrive's API server should run on. Note this is bound to the local machine only; it cannot be accessed by other machines.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]uint16{
				config.Network_All: DefaultApiPort,
			},
		},

		Network: config.Parameter[config.Network]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.NetworkID,
				Name:               "Network",
				Description:        "The Ethereum network you want to use - select Hoodi Testnet to practice with fake ETH, or Mainnet to stake on the real network using real ETH.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon, config.ContainerID_ExecutionClient, config.ContainerID_BeaconNode, config.ContainerID_ValidatorClient, config.ContainerID_MevBoost},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Options: getNetworkOptions(networks),
			Default: map[config.Network]config.Network{
				config.Network_All: selectedNetwork,
			},
		},

		EnableIPv6: config.Parameter[bool]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.EnableIPv6ID,
				Name:               "Enable IPv6",
				Description:        "Enable IPv6 networking for Hyperdrive services. This is useful if you have an IPv6 network and want to use it for Hyperdrive.\n\nIf this isn't the first time you're starting Hyperdrive, you'll have to recreate the network after changing this box with `hyperdrive service down` and `hyperdrive service start` for it to take effect.\n\n[orange]NOTE: For IPv6 support to work, you must manually set up your Docker daemon to support it. Please follow the instructions at https://docs.docker.com/config/daemon/ipv6/#dynamic-ipv6-subnet-allocation before checking this box.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_BeaconNode, config.ContainerID_Daemon, config.ContainerID_ExecutionClient, config.ContainerID_Exporter, config.ContainerID_Grafana, config.ContainerID_Prometheus, config.ContainerID_ValidatorClient},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]bool{
				config.Network_All: false,
			},
		},

		ClientMode: config.Parameter[config.ClientMode]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.ClientModeID,
				Name:               "Client Mode",
				Description:        "Choose which mode to use for your Execution Client and Beacon Node - locally managed (Docker Mode), or externally managed (Hybrid Mode).",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon, config.ContainerID_ExecutionClient, config.ContainerID_BeaconNode},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Options: []*config.ParameterOption[config.ClientMode]{
				{
					ParameterOptionCommon: &config.ParameterOptionCommon{
						Name:        "Locally Managed",
						Description: "Allow Hyperdrive to manage the Execution Client and Beacon Node for you (Docker Mode)",
					},
					Value: config.ClientMode_Local,
				}, {
					ParameterOptionCommon: &config.ParameterOptionCommon{
						Name:        "Externally Managed",
						Description: "Use an existing Execution Client and Beacon Node that you manage on your own (Hybrid Mode)",
					},
					Value: config.ClientMode_External,
				}},
			Default: map[config.Network]config.ClientMode{
				config.Network_All: config.ClientMode_Local,
			},
		},

		AutoTxMaxFee: config.Parameter[float64]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.AutoTxMaxFeeID,
				Name:               "Auto TX Max Fee",
				Description:        "Set this if you want all of Hyperdrive's automatic transactions to use this specific max fee value (in gwei), which is the most you'd be willing to pay (*including the priority fee*).\n\nA value of 0 will use the suggested max fee based on the current network conditions.\n\nAny other value will ignore the network suggestion and use this value instead.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]float64{
				config.Network_All: float64(0),
			},
		},

		MaxPriorityFee: config.Parameter[float64]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.MaxPriorityFeeID,
				Name:               "Max Priority Fee",
				Description:        "The default value for the priority fee (in gwei) for all of your transactions, including automatic ones. This describes how much you're willing to pay *above the network's current base fee* - the higher this is, the more ETH you give to the validators for including your transaction, which generally means it will be included in a block faster (as long as your max fee is sufficiently high to cover the current network conditions).\n\nMust be larger than 0.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]float64{
				config.Network_All: float64(0.1),
			},
		},

		AutoTxGasThreshold: config.Parameter[float64]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.AutoTxGasThresholdID,
				Name:               "Automatic TX Gas Threshold",
				Description:        "The threshold (in gwei) that the recommended network gas price must be under in order for automated transactions to be submitted when due. A value of 0 will disable non-essential automatic transactions.\n\nNOTE: If Auto TX Max Fee is set, this setting will be ignored.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]float64{
				config.Network_All: float64(8),
			},
		},

		UserDataPath: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.UserDataPathID,
				Name:               "User Data Path",
				Description:        "The absolute path of your personal `data` folder that contains secrets such as your node wallet's encrypted file, the password for your node wallet, and all of the validator keys for any Hyperdrive modules.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon, config.ContainerID_ValidatorClient},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]string{
				config.Network_All: filepath.Join(hdDir, "data"),
			},
		},

		AdditionalDockerNetworks: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.AdditionalDockerNetworksID,
				Name:               "Additional Docker Networks",
				Description:        "List any other externally-managed Docker networks running on this machine that you'd like to give the Hyperdrive services access to here. Use a comma-separated list of network names.\n\nTo get a list of local Docker networks, run `docker network ls`.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_BeaconNode, config.ContainerID_Daemon, config.ContainerID_ExecutionClient, config.ContainerID_Grafana, config.ContainerID_Prometheus, config.ContainerID_ValidatorClient},
				CanBeBlank:         true,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]string{
				config.Network_All: "",
			},
		},

		ClientTimeout: config.Parameter[uint16]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.ClientTimeoutID,
				Name:               "Client Timeout",
				Description:        "The maximum time (in seconds) that Hyperdrive will wait for a response during HTTP requests (such as Execution Client, Beacon Node, or nodeset.io requests) before timing out.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon},
				CanBeBlank:         false,
				OverwriteOnUpgrade: false,
			},
			Default: map[config.Network]uint16{
				config.Network_All: 30,
			},
		},

		ContainerTag: config.Parameter[string]{
			ParameterCommon: &config.ParameterCommon{
				ID:                 ids.ContainerTagID,
				Name:               "Daemon Container Tag",
				Description:        "The tag name of the Hyperdrive Daemon image to use.",
				AffectsContainers:  []config.ContainerID{config.ContainerID_Daemon},
				CanBeBlank:         false,
				OverwriteOnUpgrade: true,
			},
			Default: map[config.Network]string{
				config.Network_All: hyperdriveTag,
			},
		},
	}

	// Create the subconfigs
	cfg.Logging = config.NewLoggerConfig()
	cfg.LocalExecutionClient = config.NewLocalExecutionConfig()
	cfg.ExternalExecutionClient = config.NewExternalExecutionConfig()
	cfg.LocalBeaconClient = config.NewLocalBeaconConfig()
	cfg.ExternalBeaconClient = config.NewExternalBeaconConfig()
	cfg.Fallback = config.NewFallbackConfig()
	cfg.Metrics = NewMetricsConfig()
	cfg.MevBoost = NewMevBoostConfig(cfg)

	// Provision the defaults for each network
	for _, network := range networks {
		err := config.SetDefaultsForNetworks(cfg, network.DefaultConfigSettings, network.Key)
		if err != nil {
			return nil, fmt.Errorf("could not set defaults for network %s: %w", network.Key, err)
		}
	}

	// Apply the default values for the network
	cfg.Network.Value = selectedNetwork
	cfg.applyAllDefaults()

	return cfg, nil
}

// Get the title for this config
func (cfg *HyperdriveConfig) GetTitle() string {
	return "Hyperdrive"
}

// Get the config.Parameters for this config
func (cfg *HyperdriveConfig) GetParameters() []config.IParameter {
	return []config.IParameter{
		&cfg.ProjectName,
		&cfg.ApiPort,
		&cfg.Network,
		&cfg.EnableIPv6,
		&cfg.ClientMode,
		&cfg.AutoTxMaxFee,
		&cfg.MaxPriorityFee,
		&cfg.AutoTxGasThreshold,
		&cfg.UserDataPath,
		&cfg.AdditionalDockerNetworks,
		&cfg.ClientTimeout,
		&cfg.ContainerTag,
	}
}

// Get the subconfigurations for this config
func (cfg *HyperdriveConfig) GetSubconfigs() map[string]config.IConfigSection {
	return map[string]config.IConfigSection{
		ids.LoggingID:           cfg.Logging,
		ids.FallbackID:          cfg.Fallback,
		ids.LocalExecutionID:    cfg.LocalExecutionClient,
		ids.ExternalExecutionID: cfg.ExternalExecutionClient,
		ids.LocalBeaconID:       cfg.LocalBeaconClient,
		ids.ExternalBeaconID:    cfg.ExternalBeaconClient,
		ids.MetricsID:           cfg.Metrics,
		ids.MevBoostID:          cfg.MevBoost,
	}
}

// Serializes the configuration into a map of maps, compatible with a settings file
func (cfg *HyperdriveConfig) Serialize(modules []IModuleConfig, includeUserDir bool) map[string]any {
	masterMap := map[string]any{}

	hdMap := config.Serialize(cfg)
	masterMap[ids.VersionID] = fmt.Sprintf("v%s", shared.HyperdriveVersion)
	masterMap[ids.RootConfigID] = hdMap

	if includeUserDir {
		masterMap[ids.UserDirID] = cfg.hyperdriveUserDirectory
	}

	// Handle modules
	modulesMap := map[string]any{}
	for modName, value := range cfg.Modules {
		// Copy the module configs already on-board
		modulesMap[modName] = value
	}
	for _, module := range modules {
		// Serialize / overwrite them with explictly provided ones
		modMap := module.Serialize()
		modulesMap[module.GetModuleName()] = modMap
	}
	masterMap[ModulesName] = modulesMap
	return masterMap
}

// Deserializes a settings file into this config
func (cfg *HyperdriveConfig) Deserialize(masterMap map[string]any) error {
	// Upgrade the config to the latest version
	err := migration.UpdateConfig(masterMap)
	if err != nil {
		return fmt.Errorf("error upgrading configuration to v%s: %w", shared.HyperdriveVersion, err)
	}

	// Get the network
	network := config.Network_Mainnet
	hyperdriveParams, exists := masterMap[ids.RootConfigID]
	if !exists {
		return fmt.Errorf("config is missing the [%s] section", ids.RootConfigID)
	}
	hdMap, isMap := hyperdriveParams.(map[string]any)
	if !isMap {
		return fmt.Errorf("config has an entry named [%s] but it is not a map, it's a %s", ids.RootConfigID, reflect.TypeOf(hyperdriveParams))
	}
	networkVal, exists := hdMap[cfg.Network.ID]
	if exists {
		networkString, isString := networkVal.(string)
		if !isString {
			return fmt.Errorf("expected [%s - %s] to be a string but it is not", ids.RootConfigID, cfg.Network.ID)
		}
		network = config.Network(networkString)
	}

	// Deserialize the params and subconfigs
	err = config.Deserialize(cfg, hdMap, network)
	if err != nil {
		return fmt.Errorf("error deserializing [%s]: %w", ids.RootConfigID, err)
	}

	// Get the special fields
	version, exists := masterMap[ids.VersionID]
	if !exists {
		return fmt.Errorf("expected a version config.Parameter named [%s] but it was not found", ids.VersionID)
	}
	cfg.Version = version.(string)
	userDir, exists := masterMap[ids.UserDirID]
	if exists {
		cfg.hyperdriveUserDirectory = userDir.(string)
	}

	// Handle modules
	modules, exists := masterMap[ModulesName]
	if exists {
		if modMap, ok := modules.(map[string]any); ok {
			cfg.Modules = modMap
		} else {
			return fmt.Errorf("config has an entry named [%s] but it is not a map, it's a %s", ModulesName, reflect.TypeOf(modules))
		}
	} else {
		cfg.Modules = map[string]any{}
	}

	return nil
}

// Changes the current network, propagating new parameter settings if they are affected
func (cfg *HyperdriveConfig) ChangeNetwork(newNetwork config.Network) {
	// Get the current network
	oldNetwork := cfg.Network.Value
	if oldNetwork == newNetwork {
		return
	}
	cfg.Network.Value = newNetwork

	// Run the changes
	config.ChangeNetwork(cfg, oldNetwork, newNetwork)
}

// Creates a copy of the configuration
func (cfg *HyperdriveConfig) Clone() *HyperdriveConfig {
	clone, _ := NewHyperdriveConfig(cfg.hyperdriveUserDirectory, cfg.networkSettings)
	config.Clone(cfg, clone, cfg.Network.Value)
	clone.Version = cfg.Version
	return clone
}

// =====================
// === Field Helpers ===
// =====================

// Applies all of the defaults to all of the settings that have them defined
func (cfg *HyperdriveConfig) applyAllDefaults() {
	network := cfg.Network.Value
	config.ApplyDefaults(cfg, network)
}

// Get the list of options for networks to run on
func getNetworkOptions(networks []*HyperdriveSettings) []*config.ParameterOption[config.Network] {
	// Create the options
	options := []*config.ParameterOption[config.Network]{}
	for _, network := range networks {
		options = append(options, &config.ParameterOption[config.Network]{
			ParameterOptionCommon: &config.ParameterOptionCommon{
				Name:        network.Name,
				Description: network.Description,
			},
			Value: network.Key,
		})
	}

	// Sort the options so mainnet comes first and Hoodi comes second
	sort.SliceStable(options, func(i, j int) bool {
		firstOption := options[i]
		secondOption := options[j]

		// Mainnet comes first
		if firstOption.Value == config.Network_Mainnet {
			return true
		}
		if secondOption.Value == config.Network_Mainnet {
			return false
		}

		// Hoodi comes second
		if firstOption.Value == config.Network_Hoodi {
			return true
		}
		if secondOption.Value == config.Network_Hoodi {
			return false
		}

		// The rest doesn't matter so just sort it alphabetically
		return firstOption.Value < secondOption.Value
	})

	return options
}

// Get the Eth network name of the selected network
func (cfg *HyperdriveConfig) GetEthNetworkName() string {
	for _, network := range cfg.networkSettings {
		if network.Key == cfg.Network.Value {
			return network.NetworkResources.EthNetworkName
		}
	}
	return ""
}

// Get all loaded network settings
func (cfg *HyperdriveConfig) GetNetworkSettings() []*HyperdriveSettings {
	return cfg.networkSettings
}

func (cfg *HyperdriveConfig) GetUserDirectory() string {
	return cfg.hyperdriveUserDirectory
}

// ==============================
// === IConfig Implementation ===
// ==============================

func (cfg *HyperdriveConfig) GetApiLogFilePath() string {
	return filepath.Join(cfg.hyperdriveUserDirectory, LogDir, ApiLogName)
}

func (cfg *HyperdriveConfig) GetTasksLogFilePath() string {
	return filepath.Join(cfg.hyperdriveUserDirectory, LogDir, TasksLogName)
}

func (cfg *HyperdriveConfig) GetNodeAddressFilePath() string {
	return filepath.Join(cfg.UserDataPath.Value, UserAddressFilename)
}

func (cfg *HyperdriveConfig) GetWalletFilePath() string {
	return filepath.Join(cfg.UserDataPath.Value, UserWalletDataFilename)
}

func (cfg *HyperdriveConfig) GetPasswordFilePath() string {
	return filepath.Join(cfg.UserDataPath.Value, UserPasswordFilename)
}

func (cfg *HyperdriveConfig) GetExecutionClientUrls() (string, string) {
	primaryEcUrl := cfg.GetEcHttpEndpoint()
	var fallbackEcUrl string
	if cfg.Fallback.UseFallbackClients.Value {
		fallbackEcUrl = cfg.Fallback.EcHttpUrl.Value
	}
	return primaryEcUrl, fallbackEcUrl
}

func (cfg *HyperdriveConfig) GetBeaconNodeUrls() (string, string) {
	primaryBnUrl := cfg.GetBnHttpEndpoint()
	var fallbackBnUrl string
	if cfg.Fallback.UseFallbackClients.Value {
		fallbackBnUrl = cfg.Fallback.BnHttpUrl.Value
	}
	return primaryBnUrl, fallbackBnUrl
}

func (cfg *HyperdriveConfig) GetLoggerOptions() log.LoggerOptions {
	return cfg.Logging.GetOptions()
}

func (cfg *HyperdriveConfig) GetModuleLogFilePath(moduleName string, moduleLogName string) string {
	return filepath.Join(cfg.hyperdriveUserDirectory, LogDir, moduleName, moduleLogName)
}
