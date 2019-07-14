package gsmtcp

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// OpenTcpConnection attempts to establish a new connection to the given IP and port.
func (g *DefaultGsmModule) OpenTcpConnection(address string) error {
	addressParts := strings.Split(address, ":")
	ip := strings.TrimSpace(addressParts[0])
	port := strings.TrimSpace(addressParts[1])
	connStr := fmt.Sprintf(string(ConnectCommand), ip, port)
	// send the connect command
	err := g.sp.Println(connStr)
	if err != nil {
		return errors.New("could not open connection:" + err.Error())
	}
	// First phase
	log.Debug().Msg("waiting for OK")
	m, err := g.sp.WaitForRegexTimeout(
		fmt.Sprintf("%s|%s",
			string(OkResponse),
			string(ErrorResponse)), 5*time.Second)
	if err != nil {
		return err
	}
	if m == string(OkResponse) {
		// Second phase
		log.Debug().Msg("waiting for CONNECT OK")
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
		log.Debug().Msg("waiting for CONNECT FAIL or TCP CLOSED")
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

// GetLocalIPAddress
func (g *DefaultGsmModule) GetLocalIPAddress() (string, error) {
	err := g.sp.Println(string(GetLocalIPAddressCommand))
	if err != nil {
		log.Error().Err(err)
		return "", err
	}
	ip, err := g.sp.WaitForRegexTimeout("[0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}", 3*time.Second)
	if err != nil {
		return "", err
	}
	return ip, nil
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

// SendRawTcpData sends the given data to to open connection.
func (g *DefaultGsmModule) SendRawTcpData(data []byte) (int, error) {
	sendTimeout := time.Duration(getConfigValue(SendTimeoutConfig, g.configs...).(SendTimeout))
	err := g.sp.Println(fmt.Sprintf("%s?", string(SendCommand)))
	match, err := g.sp.WaitForRegexTimeout("\\+CIPSEND: [0-9]+", sendTimeout)
	if err != nil {
		return -1, err
	}
	matches := regexp.MustCompile("\\+CIPSEND: ([0-9]+)").FindAllStringSubmatch(match, -1)
	maxBytes, err := strconv.Atoi(matches[0][1])
	if err != nil {
		log.Error().Err(err)
		return -1, err
	}
	bytesToWrite := len(data)
	dataToWrite := data[:]
	maxBytesReached := false
	if len(data) > maxBytes {
		bytesToWrite = maxBytes
		dataToWrite = data[:maxBytes]
		maxBytesReached = true
	}
	// send the 'send' command
	err = g.sp.Println(fmt.Sprintf("%s=%d", string(SendCommand), bytesToWrite))
	if err != nil {
		return -1, err
	}
	time.Sleep(10 * time.Millisecond)
	sendData := append(dataToWrite)
	// send the actual data
	_, err = g.sp.Write(sendData)
	if err != nil {
		return -1, err
	}
	_, err = g.sp.WaitForRegexTimeout(fmt.Sprintf("%s",
		string(SendOkResponse)), sendTimeout)
	if err != nil {
		return -1, err
	}
	if maxBytesReached {
		return bytesToWrite, MaxBytesErr{}
	}
	return bytesToWrite, err
}

func (g *DefaultGsmModule) ReadData() (byte, error) {
	return g.sp.Read()
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
