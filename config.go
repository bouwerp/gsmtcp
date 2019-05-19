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
			return NetworkRegistrationRetryDelay(0).Default()
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
	return 15
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
	return 3 * time.Second
}
