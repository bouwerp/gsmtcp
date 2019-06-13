# gsmtcp
A golang library for TCP communication over an embedded GSM device. It was 
primarily developed with the SIM800 series of devices interfacing with Raspberry Pi, and is still very much
a work in progress.

## Usage example

When working with a Raspberry Pi, for example, the peripheral drivers
must first be initialised. Utilising [periph.io](https://periph.io):

```go
pState, err := host.Init()
if err != nil {
    log.Error(err)
    os.Exit(1)
}
for _, d := range pState.Failed {
    log.Warn("failed to load", d.String(), ":", d.Err.Error())
}
```
The GSM module can then be created and initialised:
```go
g, err := gsm.NewGsmModule("/dev/ttyS0")
if err != nil {
    log.Error(err)
    return
}

err = g.Init()
if err != nil {
    log.Error(err)
    return
}
defer func() {
    err := g.Shutdown()
    if err != nil {
        log.Error(err)
    }
}()
```

A TCP connection can now be created:
```go
conn, err := gsm.NewConnection(g, "<IPv4>", "<PORT")
if err != nil {
    log.Error("could not establish a new connection", err)
    os.Exit(1)
}
defer func() {
    err := conn.Close()
    if err != nil {
        log.Error(err.Error())
    }
    time.Sleep(1 * time.Second)
}()
```

## Establishing a TLS connection

A secure connection can be established by utilising _golang_'s standard libraries:

```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{*cert},
    VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
        for _, chain := range verifiedChains {
            for _, c := range chain {
                // peer certificate verification
            }
        }
        return nil
    },
    InsecureSkipVerify: true,
    Rand:               rand.Reader,
    MinVersion:         tls.VersionTLS12,
}

tlsConn := tls.Client(conn, tlsConfig)
defer func() {
    err := tlsConn.Close()
    if err != nil {
        log.Error(err)
    }
}()

// perform the handshake
err = tlsConn.Handshake()
if err != nil {
    log.Error(err)
}
```

The certificate (`*cert`) can be generated with the help of [Talisman](https://github.com/bouwerp/talisman).