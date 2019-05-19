package gsmtcp

import (
	"errors"
	"fmt"
	"github.com/argandas/serial"
	"github.com/bouwerp/log"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"regexp"
	"time"
)

// NewGsmModule opens a serial connection to the provided serial device.
func NewGsmModule(device string, configs ...Config) (*DefaultGsmModule, error) {
	// open the serial port
	sp := serial.New()
	baudConfig := getConfigValue(BaudConfig, configs...)
	err := sp.Open(device, baudConfig.(int))
	if err != nil {
		return nil, err
	}
	sp.Verbose = false
	return &DefaultGsmModule{
		device:  device,
		sp:      sp,
		configs: configs,
	}, nil
}

// Init checks the GSM module status, and switches it on if it was off; It then waits for network registration.
func (g *DefaultGsmModule) Init() error {
	log.Debug("checking GSM module status")
	on, err := g.GetStatus()
	if err != nil {
		return err
	}
	if !on {
		log.Debug("GSM module is OFF - switching it on")
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
		log.Debug("GMS module is ON")
	}

	err = g.WaitForNetworkRegistration()
	if err != nil {
		return err
	}
	log.Debug("registered with network")
	return nil
}

// Shutdown switches the GSM module off.
func (g *DefaultGsmModule) Shutdown() error {
	// toggle the SIM868 module
	log.Debug("switching SIM868 module OFF")
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
		log.Error("could not close serial device", g.device, ":", err)
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

// OpenTcpConnection attempts to establish a new connection to the given IP and port.
func (g *DefaultGsmModule) OpenTcpConnection(ip, port string) error {
	connStr := fmt.Sprintf(string(ConnectCommand), ip, port)
	// send the connect command
	err := g.sp.Println(connStr)
	if err != nil {
		return errors.New("could not open connection:" + err.Error())
	}
	// First phase
	log.Debug("waiting for OK")
	m, err := g.sp.WaitForRegexTimeout(
		fmt.Sprintf("%s|%s",
			string(OkResponse),
			string(ErrorResponse)), 5*time.Second)
	if err != nil {
		return err
	}
	if m == string(OkResponse) {
		// Second phase
		log.Debug("waiting for CONNECT OK")
		m, err = g.sp.WaitForRegexTimeout(
			fmt.Sprintf("%s|%s|%s",
				string(ConnectOkResponse),
				string(AlreadyConnectedResponse),
				string(StateTcpClosedResponse)), 5*time.Second)
		if err != nil {
			return err
		}
		if m == string(ConnectOkResponse) || m == string(AlreadyConnectedResponse) {
			return nil
		} else {
			return errors.New(m)
		}
	} else {
		// Second phase
		log.Debug("waiting for CONNECT FAIL or TCP CLOSED")
		m, err = g.sp.WaitForRegexTimeout(
			fmt.Sprintf("%s|%s",
				string(ConnectFailedResponse),
				string(StateTcpClosedResponse)), 5*time.Second)
		if err != nil {
			return err
		}
		return errors.New(m)
	}
}

// commandEchoOff turns off the echoing of commands
func commandEchoOff(s serial.SerialPort) error {
	// send the connect command
	err := s.Println(string(EchoOffCommand))
	if err != nil {
		return errors.New("could not close connection:" + err.Error())
	}
	m, err := s.WaitForRegexTimeout(fmt.Sprintf("%s|%s",
		string(OkResponse),
		string(ErrorResponse)), 3*time.Second)
	if err != nil {
		return err
	}
	if m != string(OkResponse) {
		return errors.New(m)
	}
	return nil
}

// CloseTcpConnection closes the current connection.
func (g *DefaultGsmModule) CloseTcpConnection() error {
	// send the connect command
	err := g.sp.Println(string(DisconnectCommand))
	if err != nil {
		return errors.New("could not close connection:" + err.Error())
	}
	m, err := g.sp.WaitForRegexTimeout(fmt.Sprintf("%s|%s",
		string(CloseOkResponse),
		string(ErrorResponse)), 3*time.Second)
	if err != nil {
		return err
	}
	if m != string(CloseOkResponse) {
		return errors.New(m)
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

// SendRawTcpData sends the given data to to open connection.
func (g *DefaultGsmModule) SendRawTcpData(data []byte) error {
	// send the connect command
	err := g.sp.Println(fmt.Sprintf("%s=%d", string(SendCommand), len(data)))
	if err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	sendData := append(data)
	// send the actual data
	_, err = g.sp.Write(sendData)
	if err != nil {
		return err
	}
	_, err = g.sp.WaitForRegexTimeout(fmt.Sprintf("%s",
		string(SendOkResponse)), 5*time.Second)
	if err != nil {
		return err
	}
	return nil
}

func (g *DefaultGsmModule) ReadData() (byte, error) {
	return g.sp.Read()
}

// IsConnected determines if a connection is currently established.
func (g *DefaultGsmModule) IsConnected() (bool, error) {
	// send the connect command
	err := g.sp.Println(string(ConnectionStateCommand))
	if err != nil {
		return false, errors.New("could not determine connection state:" + err.Error())
	}
	// First phase
	m, err := g.sp.WaitForRegexTimeout(
		fmt.Sprintf("%s|%s",
			string(OkResponse),
			string(ErrorResponse)), 5*time.Second)
	if err != nil {
		return false, err
	}
	if m == string(OkResponse) {
		m, err = g.sp.WaitForRegexTimeout(
			fmt.Sprintf("%s",
				string(StateConnectOkResponse)), 5*time.Second)
		if err != nil {
			return false, err
		}
		return true, nil
	} else {
		return false, errors.New(m)
	}
}

// GetStatus determines the status of the module.
func (g *DefaultGsmModule) GetStatus() (bool, error) {
	err := g.sp.Println(string(StatusCommand))
	if err != nil {
		log.Debug("GetStatus", "Println", err.Error())
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

// GetLocalIPAddress
func (g *DefaultGsmModule) GetLocalIPAddress() (string, error) {
	err := g.sp.Println(string(GetLocalIPAddressCommand))
	if err != nil {
		log.Debug("GetLocalIPAddress", "Println", err.Error())
		return "", err
	}
	ip, err := g.sp.WaitForRegexTimeout("[0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}", 3*time.Second)
	if err != nil {
		return "", err
	}
	return ip, nil
}

// ToggleModule toggles the PWRKEY pin of the module.
func (g *DefaultGsmModule) ToggleModule() error {
	log.Debug("toggling SIM868")
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
