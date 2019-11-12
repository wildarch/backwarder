package main

import (
    "log"
    "github.com/gorilla/websocket"
    "flag"
    "net/http"
    "io"
)

var bindAddr = flag.String("bind_addr", "localhost:8888", "Server bind address")
var backconnectPath = flag.String("backconnect_path", "/backconnect", "URL path to connect back via websocket")
var upgrader = websocket.Upgrader{}
// TODO this global variable to should be put somewhere nicer.
var pool = make(chan *websocket.Conn)

func backconnectRegister(w http.ResponseWriter, r *http.Request) {
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("Backconnect request upgrade failed: ", err)
        return
    }
    pool <- c
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
    ws:=  <-pool
    var doneWaitGroup sync.WaitGroup
    doneWaitGroup.Add(2)
    // Write incoming request to websocket
    go func() {
        w, e := ws.NextWriter(websocket.BinaryMessage)
        defer w.Close()
        if e != nil {
            log.Println("Could not obtain a writer for the request: ", e)
        } else {
            r.WriteProxy(w)
        }
        doneWaitGroup.Done()
    }()
    // Write data returned from the websocket
    go func() {
        for {
            t, r, e := ws.NextReader()
            if e != nil {
                log.Println("Could not obtain a reader from the websocket:", e)
                break
            }
            if t != websocket.BinaryMessage {
                log.Println("Reader for unsupported message type: ", t)
                break
            }
            io.Copy(w, r)
        }
        doneWaitGroup.Done()
    }()
    doneWaitGroup.Wait()
    log.Println("Request served")
}

func main() {
    log.Println("Hello, world!")
    http.HandleFunc(*backconnectPath, backconnectRegister)
    http.HandleFunc("/", handleRequest)
    log.Fatal(http.ListenAndServe(*bindAddr,  nil))
}
