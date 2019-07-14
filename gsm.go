package gsmtcp

import (
	"errors"
	"fmt"
	"github.com/argandas/serial"
	"github.com/rs/zerolog/log"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"regexp"
	"time"
)

// NewGsmModule opens a serial connection to the provided serial device.
func NewGsmModule(device string, configs ...Config) (*DefaultGsmModule, error) {
	verbose := getConfigValue(VerboseConfig, configs...).(Verbose)
	// open the serial port
	sp := serial.New()
	baudConfig := getConfigValue(BaudConfig, configs...)
	err := sp.Open(device, baudConfig.(int))
	if err != nil {
		return nil, err
	}
	sp.Verbose = bool(verbose)
	g := &DefaultGsmModule{
		device:  device,
		sp:      sp,
		configs: configs,
	}
	return g, nil
}

// Init checks the GSM module status, and switches it on if it was off; It then waits for network registration.
func (g *DefaultGsmModule) Init() error {
	//apn := getConfigValue(APNConfig, g.configs...).(APN)
	log.Debug().Msg("checking GSM module status")
	on, err := g.GetStatus()
	if err != nil {
		return err
	}
	if !on {
		log.Debug().Msg("GSM module is OFF - switching it on")
		// toggle the SIM868 module
		err := g.ToggleModule()
		if err != nil {
			return err
		}
		on, err = g.GetStatus()
		if err != nil {
			return err
		}
		if !on {
			return err
		}
	} else {
		log.Debug().Msg("GMS module is ON")
	}
	err = g.CommandEchoOff()
	if err != nil {
		return err
	}
	err = g.WaitForNetworkRegistration()
	if err != nil {
		return err
	}
	log.Debug().Msg("registered with network")
	return nil
}

// Shutdown switches the GSM module off.
func (g *DefaultGsmModule) Shutdown() error {
	// toggle the SIM868 module
	log.Debug().Msg("switching SIM868 module OFF")
	err := g.ToggleModule()
	if err != nil {
		return err
	}
	off, err := g.GetStatus()
	if err != nil {
		return err
	}
	if !off {
		return errors.New("GSM module not off")
	}
	g.CloseGsmModule()
	time.Sleep(1 * time.Second)
	return nil
}

// CloseGsmModule closes the serial connection to the GSM module.
func (g *DefaultGsmModule) CloseGsmModule() {
	err := g.sp.Close()
	if err != nil {
		log.Error().Err(err).Msgf("could not close serial device %s", g.device)
	}
}

type DefaultGsmModule struct {
	sp            *serial.SerialPort
	device        string
	configs       []Config
	TotalDeadline time.Time
	ReadDeadline  time.Time
	WriteDeadline time.Time
}

type Command string

const StatusCommand Command = `AT`
const ConnectCommand Command = `AT+CIPSTART="TCP", "%s", "%s"`
const DisconnectCommand Command = `AT+CIPCLOSE`
const SendCommand Command = `AT+CIPSEND`
const ConnectionStateCommand Command = `AT+CIPSTATUS`
const CheckNetworkRegistrationCommand Command = `AT+CGREG?`
const GetLocalIPAddressCommand Command = `AT+CIFSR`
const EchoOffCommand Command = `ATE0`
const EchoOnCommand Command = `ATE1`

type ResponseMessage string

const OkResponse ResponseMessage = "OK"
const ErrorResponse ResponseMessage = "ERROR"
const ConnectOkResponse ResponseMessage = "CONNECT OK"
const CloseOkResponse ResponseMessage = "CLOSE OK"
const AlreadyConnectedResponse ResponseMessage = "ALREADY CONNECT"
const StateTcpClosedResponse ResponseMessage = "STATE: TCP CLOSED"
const StateConnectOkResponse ResponseMessage = "STATE: CONNECT OK"
const ConnectFailedResponse ResponseMessage = "CONNECT FAIL"
const SendOkResponse ResponseMessage = "SEND OK"

type NetworkRegistrationStatus string

const NotRegistered NetworkRegistrationStatus = "0"
const RegisteredHome NetworkRegistrationStatus = "1"
const TryingToRegister NetworkRegistrationStatus = "2"
const RegistrationDenied NetworkRegistrationStatus = "3"
const UnknownRegistrationError NetworkRegistrationStatus = "4"
const RegisteredRoaming NetworkRegistrationStatus = "5"

func (g *DefaultGsmModule) executeATCommand(cmd string) error {
	err := g.sp.Println(cmd)
	if err != nil {
		return errors.New("could set multi connection:" + err.Error())
	}
	log.Debug().Msg("waiting for OK")
	m, err := g.sp.WaitForRegexTimeout(
		fmt.Sprintf("%s|%s",
			string(OkResponse),
			string(ErrorResponse)), 5*time.Second)
	if err != nil {
		log.Error().Msgf(err.Error())
		return err
	}
	if m == string(OkResponse) {
		return nil
	}
	log.Error().Msgf(m)
	return errors.New(m)
}

// CommandEchoOff turns off the echoing of commands
func (g *DefaultGsmModule) CommandEchoOff() error {
	// send the connect command
	err := g.executeATCommand(string(EchoOffCommand))
	if err != nil {
		return errors.New("could not turn command echo off:" + err.Error())
	}
	return nil
}

// CommandEchoOn turns on the echoing of commands
func (g *DefaultGsmModule) CommandEchoOn() error {
	// send the connect command
	err := g.executeATCommand(string(EchoOnCommand))
	if err != nil {
		return errors.New("could not turn command echo on:" + err.Error())
	}
	return nil
}

// WaitForNetworkRegistration waits for the GSM module to be registered with the network.
func (g *DefaultGsmModule) WaitForNetworkRegistration() error {
	maxRetries := getConfigValue(NetworkRegistrationRetriesConfig, g.configs...)
	maxRetryDelay := getConfigValue(NetworkRegistrationRetryDelayConfig, g.configs...)
	retries := 0
	for {
		err := g.sp.Println(string(CheckNetworkRegistrationCommand))
		if err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
		m, err := g.sp.WaitForRegexTimeout(fmt.Sprintf("%s",
			"[+]CGREG[:] .,."), 10*time.Second)
		if err != nil {
			return err
		}
		s := regexp.MustCompile("[+]CGREG: [0-9],([0-9])").FindAllStringSubmatch(m, -1)
		registrationStatus := NetworkRegistrationStatus(s[0][1])
		switch registrationStatus {
		case TryingToRegister, NotRegistered:
			if retries == int(maxRetries.(NetworkRegistrationRetries)) {
				return errors.New("maximum retries for registering to network")
			}
			time.Sleep(time.Duration(maxRetryDelay.(NetworkRegistrationRetryDelay)))
			retries++
			continue
		case RegistrationDenied, UnknownRegistrationError:
			return errors.New("error registering to network: " + string(registrationStatus))
		case RegisteredRoaming, RegisteredHome:
			return nil
		default:
			return errors.New("unknown registration status" + string(registrationStatus))
		}
	}
}

// GetStatus determines the status of the module.
func (g *DefaultGsmModule) GetStatus() (bool, error) {
	err := g.sp.Println(string(StatusCommand))
	if err != nil {
		log.Error().Err(err)
		return false, err
	}
	_, err = g.sp.WaitForRegexTimeout(string(OkResponse), 5*time.Second)
	if err != nil {
		if err.Error() == "Timeout expired" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ToggleModule toggles the PWRKEY pin of the module.
func (g *DefaultGsmModule) ToggleModule() error {
	log.Debug().Msg("toggling SIM868")
	err := gpioreg.ByName("GPIO4").Out(gpio.Low)
	if err != nil {
		return errors.New(err.Error())
	}
	time.Sleep(4 * time.Second)
	err = gpioreg.ByName("GPIO4").Out(gpio.High)
	if err != nil {
		log.Error()
		return errors.New(err.Error())
	}
	return nil
}
