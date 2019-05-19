# gsmtcp
A golang library for TCP communication over an embedded GSM device. It was 
primarily developed with the SIM800 series of devices interfacing with Raspberry Pi, and is still very much
a work in progress.

# Usage example

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
