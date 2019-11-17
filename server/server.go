package backwarder

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
		log.Printf("Incoming request for: %s", r.URL)
    ws:=  <-pool

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
						log.Printf("Written response to websocket")
        }
    }()

		// Write the request to the websocket
		wsW, e := ws.NextWriter(websocket.BinaryMessage)
		defer wsW.Close()
		if e != nil {
				log.Println("Could not obtain a writer for the request: ", e)
		} else {
				r.WriteProxy(wsW)
		}
		log.Printf("Written request to websocket")
}

func GetServeMux(backconnectPath string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(backconnectPath, backconnectRegister)
	mux.HandleFunc("/", handleRequest)
	return mux
}

func main() {
    log.Println("Hello, world!")
    log.Fatal(http.ListenAndServe(*bindAddr,  GetServeMux(*backconnectPath)))
}
