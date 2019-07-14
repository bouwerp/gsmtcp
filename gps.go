package gsmtcp

import "errors"

func (g *DefaultGsmModule) SwitchGNSSPowerOn() error {
	err := g.executeATCommand("AT+CGNSPWR=1")
	if err != nil {
		return errors.New("could set switch GNSS power on:" + err.Error())
	}
	return nil
}
func (g *DefaultGsmModule) SwitchGNSSPowerOff() error {
	err := g.executeATCommand("AT+CGNSPWR=0")
	if err != nil {
		return errors.New("could set switch GNSS power on:" + err.Error())
	}
	return nil
}
