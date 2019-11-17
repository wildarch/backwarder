package backwarder

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"time"
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

func getHealthySocket(pool chan *websocket.Conn) *websocket.Conn {
	for {
		ws := <-pool

		// Send a ping to test the connection works
		e := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
		if e != nil {
			log.Printf("websocket unhealthy, ping failed: %s", e)
			ws.Close()
			continue
		}
		// TODO(daagra) Check for Pong.
		return ws
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Incoming request for: %s", r.URL)
	ws := getHealthySocket(pool)
	errors := make(chan error)
	done := make(chan struct{})

	// Write data returned from the websocket
	go func() {
		for {
			t, r, e := ws.NextReader()
			if e != nil {
				if !websocket.IsCloseError(e, websocket.CloseNormalClosure) {
					errors <- fmt.Errorf("Failed to read from websocket: %s", e)
				}
				break
			}
			if t != websocket.BinaryMessage {
				errors <- fmt.Errorf("Unsupported message type: %s", t)
				break
			}
			io.Copy(w, r)
			log.Printf("Written response to websocket")
		}
		done <- struct{}{}
	}()

	// Error + websocket close handler
	go func() {
		for i := 0; i < 2; i++ {
			<-done
		}
		select {
		case e := <-errors:
			// Close the websocket with the given error message
			e = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseAbnormalClosure, fmt.Sprintf("%s", e)))
			if e != nil {
				log.Printf("Failed to send abnormal close message: %s", e)
			}
		default:
			// No errors to process, normal closure
			e := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if e != nil {
				log.Printf("Failed to send normal close message: %s", e)
			}
		}
		close(errors)
		ws.Close()
	}()

	// Write the request to the websocket
	wsW, e := ws.NextWriter(websocket.BinaryMessage)
	if e != nil {
		errors <- fmt.Errorf("Failed to write to websocket: %s", e)
	}
	if e = r.WriteProxy(wsW); e != nil {
		errors <- fmt.Errorf("Failed while writing to websocket: %s", e)
	}
	wsW.Close()
	done <- struct{}{}
}

func GetServeMux(backconnectPath string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(backconnectPath, backconnectRegister)
	mux.HandleFunc("/", handleRequest)
	return mux
}

func main() {
	log.Println("Hello, world!")
	log.Fatal(http.ListenAndServe(*bindAddr, GetServeMux(*backconnectPath)))
}
