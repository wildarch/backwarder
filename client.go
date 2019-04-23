package main

import (
    "github.com/gorilla/websocket"
    "log"
    "flag"
    "net/url"
    "net"
    "io"
    "os/signal"
    "os"
    "fmt"
    "time"
)

var server = flag.String("server", "localhost:8080", "Websocket server address")
var target = flag.String("target", "localhost:8000", "Target server address")
var conns = flag.Uint("conns", 10, "Number of parallel connections")

type Tunnel struct {
    serverConn *websocket.Conn
    targetConn net.Conn
    errors chan error
    open bool
}

type tunnelCloseError struct {
    serverErr error
    targetErr error
}

func (e tunnelCloseError) Error() string {
    if e.serverErr == nil {
        return fmt.Sprintf("target connection error: %s", e.targetErr)
    }
    if e.targetErr == nil {
        return fmt.Sprintf("server connection error: %s", e.serverErr)
    }
    return fmt.Sprintf("multiple errors: [server: %s, target: %s]", e.serverErr, e.targetErr)
}

func (t *Tunnel) Close() error {
    t.open = false
    close(t.errors)
    var targetErr error = nil
    if t.targetConn != nil {
        targetErr = t.targetConn.Close()
    }
    e := tunnelCloseError {
        serverErr: t.serverConn.Close(),
        targetErr: targetErr,
    }
    if e.serverErr == nil && e.targetErr == nil {
        return nil
    }
    return e
}

func (t *Tunnel) targetToServerProxy(targetAddr string) {
    for {
        writer, err := t.serverConn.NextWriter(websocket.BinaryMessage)
        if err != nil {
            if t.Open() {
                t.errors <- err
            }
            return
        }
        _, err = io.Copy(writer, t.targetConn)
        if err != nil {
            if t.Open() {
                t.errors <- err
            }
            return
        }
    }
}

func (t *Tunnel) serverToTargetProxy(serverUrl url.URL, targetAddr string) {
    for {
        msgType, reader, err := t.serverConn.NextReader()
        if err != nil {
            if t.Open() {
                t.errors <- err
            }
            return
        }
        if msgType != websocket.BinaryMessage {
            err := fmt.Errorf("Only binary messages are supported, got type: %d", msgType)
            if t.Open() {
                t.errors <- err
            }
            return
        }
        targetConn, err := net.Dial("tcp", targetAddr)
        if err != nil {
            if t.Open() {
                t.errors <- err
            }
            return
        }
        t.targetConn = targetConn
        // Start target to server communication
        go t.targetToServerProxy(targetAddr)
        _, err = io.Copy(targetConn, reader)
        if err != nil {
            if t.Open() {
                t.errors <- err
            }
            return
        }
    }
}

func (t *Tunnel) Open() bool {
    return t.open
}

func (t *Tunnel) AwaitClose() error {
    if !t.open {
        return nil
    }
    err, more := <- t.errors
    t.Close()
    if !more {
        return nil
    }
    return err
}

func Connect(serverUrl url.URL, targetAddr string) (*Tunnel, error) {
    serverConn, _, err := websocket.DefaultDialer.Dial(serverUrl.String(), nil)
    if err != nil {
        return nil, err
    }

    tunnel := Tunnel{serverConn: serverConn, targetConn: nil, errors: make(chan error), open: true}

    go tunnel.serverToTargetProxy(serverUrl, targetAddr)

    return &tunnel, nil
}

func main() {
    flag.Parse()
    log.Printf("Proxying from %s to %s (%d parallel connections)", *server, *target, *conns);

    serverUrl := url.URL{Scheme: "ws", Host: *server, Path: "/backwarder"}

    interrupt := make(chan os.Signal, 1)
    signal.Notify(interrupt, os.Interrupt)

    tunnelErrors := make(chan error, *conns)

    tunnelCloseHandler := func (tunnel *Tunnel) {
        err := tunnel.AwaitClose()
        tunnelErrors <- err
    }

    for i := uint(0); i < *conns; i++ {
        // Fake a few clean shutdows in order to start new tunnels
        tunnelErrors <- nil
    }

    // Allow bursts up to 20 requests
    limiter := make(chan struct{}, 20)
    go func() {
        for _ = range time.Tick(10 * time.Millisecond) {
            limiter <- struct{}{}
        }
    }()

    for {
        select {
        case <- interrupt:
            log.Println("Interrupt")
            return
        case err := <- tunnelErrors:
            if err != nil {
                wsErr, ok := err.(*websocket.CloseError)
                if !ok || wsErr.Code != websocket.CloseNormalClosure {
                    log.Printf("Tunnel error: %v", err)
                } else {
                    log.Printf("Socket closed normally")
                }
            }
            go func (){
                <- limiter
                tunnel, err := Connect(serverUrl, *target)
                if err != nil {
                    tunnelErrors <- err
                    return
                }
                go tunnelCloseHandler(tunnel)
            }()
        }
    }
}
