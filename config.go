package gsmtcp

import "time"

type ConfigType string

type Config interface {
	Type() ConfigType
	Value() interface{}
	Default() interface{}
}

func generateConfigMap(configs ...Config) map[ConfigType]interface{} {
	configMap := make(map[ConfigType]interface{})
	for _, c := range configs {
		configMap[c.Type()] = c.Value()
	}
	return configMap
}

func getConfigValue(configType ConfigType, configs ...Config) interface{} {
	// parse config
	configMap := generateConfigMap(configs...)
	if value, ok := configMap[configType]; ok {
		return value
	} else {
		switch configType {
		case BaudConfig:
			return Baud(0).Default()
		case NetworkRegistrationRetryDelayConfig:
			return NetworkRegistrationRetryDelay(0).Default()
		case NetworkRegistrationRetriesConfig:
			return NetworkRegistrationRetries(0).Default()
		case SendTimeoutConfig:
			return SendTimeout(0).Default()
		case VerboseConfig:
			return Verbose(false).Default()
		default:
			return nil
		}
	}
}

type Baud int

const BaudConfig ConfigType = "Baud"

func (Baud) Default() interface{} {
	return 115200
}

func (b Baud) Type() ConfigType {
	return BaudConfig
}

func (b Baud) Value() interface{} {
	return b
}

type NetworkRegistrationRetries int

const NetworkRegistrationRetriesConfig ConfigType = "NetworkRegistrationRetriesConfig"

func (NetworkRegistrationRetries) Type() ConfigType {
	return NetworkRegistrationRetriesConfig
}

func (c NetworkRegistrationRetries) Value() interface{} {
	return c
}

func (NetworkRegistrationRetries) Default() interface{} {
	return NetworkRegistrationRetries(15)
}

type NetworkRegistrationRetryDelay time.Duration

const NetworkRegistrationRetryDelayConfig ConfigType = "NetworkRegistrationRetryDelayConfig"

func (NetworkRegistrationRetryDelay) Type() ConfigType {
	return NetworkRegistrationRetryDelayConfig
}

func (c NetworkRegistrationRetryDelay) Value() interface{} {
	return c
}

func (NetworkRegistrationRetryDelay) Default() interface{} {
	return NetworkRegistrationRetryDelay(3 * time.Second)
}

type SendTimeout time.Duration

const SendTimeoutConfig ConfigType = "SendTimeoutConfig"

func (SendTimeout) Type() ConfigType {
	return SendTimeoutConfig
}

func (c SendTimeout) Value() interface{} {
	return c
}

func (SendTimeout) Default() interface{} {
	return 10 * time.Second
}

type Verbose bool

const VerboseConfig ConfigType = "VerboseConfig"

func (Verbose) Type() ConfigType {
	return VerboseConfig
}

func (c Verbose) Value() interface{} {
	return c
}

func (Verbose) Default() interface{} {
	return Verbose(false)
}
