package backwarder

import (
	"testing"
	"net/http/httptest"
	"github.com/gorilla/websocket"
	"strings"
	"net/http"
	"time"
	"bytes"
	"bufio"
)

func TestSingleRequest(t *testing.T) {
	backwarder := httptest.NewServer(GetServeMux("/backconnect"));
	defer backwarder.Close()

	wsUrl := strings.Replace(backwarder.URL + "/backconnect", "http://", "ws://", 1)

	client, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		t.Fatalf("Couldn't open client: %s", err)
	}
	defer client.Close()
	go func() {
		msgType, reader, err := client.NextReader()
		if err != nil {
			t.Fatalf("Failed to read message on websocket: %s", err)
		}
		if msgType != websocket.BinaryMessage {
			t.Fatalf("Invalid message type (%d)", msgType)
		}
		req, err := http.ReadRequest(bufio.NewReader(reader))
		if err != nil {
			t.Fatalf("Could not parse the request sent over websocket: %s", err)
		}
		t.Logf("Received a request through the websocket: %s", req)
		if req.Method != "GET" {
			t.Fatal("Unexpected method %s", req.Method)
		}
		bodyBuf := new(bytes.Buffer)
		bodyBuf.ReadFrom(req.Body)
		body := bodyBuf.String()
		if body != "" {
			t.Fatalf("Unexpected body: '%s'", body)
		}

		res := http.Response {
			Status: "200 OK",
			StatusCode: 200,
			Proto: "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		writer, err := client.NextWriter(websocket.BinaryMessage)
		if err != nil {
			t.Fatalf("Couldn't open a writer on the websocket: %s", err)
		}
		defer writer.Close()
		res.Write(writer)
	}()

	httpClient := &http.Client {
		Timeout: time.Second * 3,
	}
	res, err := httpClient.Get(backwarder.URL)
	t.Log("Sent HTTP request")
	if err != nil {
		t.Fatalf("GET failed: %s", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("Expected OK status code, got: %d", res.StatusCode)
	}
}
