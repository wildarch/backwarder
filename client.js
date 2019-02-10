const WebSocket = require('ws');
const net = require('net');
const EventEmitter = require('events');

let tunnel_host = process.argv[2];

class Tunnel extends EventEmitter {
  constructor(socketUrl, tcpHost, tcpPort) {
    super();
    this.tcpSocket = new net.Socket();
    this.connected = false;
    this.closing = false;
    this.webSocket = new WebSocket(socketUrl);

    this.webSocket.on('message', data => {
      if (!this.connected) {
        this.tcpSocket.connect(tcpPort, tcpHost);
        this.tcpSocket.on('end', () => {
          console.log("TCP socket end");
          this.onClose();
        });
        this.tcpSocket.on('data', data => { this.webSocket.send(data); });
        this.connected = true;
      }
      this.tcpSocket.write(data);
    });

    this.webSocket.on('close', () => {
      console.log("Websocket end");
      this.onClose();
    });

    this.webSocket.on('error', (e) => {
      console.error("Websocket error: ", e);
      this.onClose();
    });
  }

  close() {
    this.tcpSocket.end();
    this.webSocket.close(1000, "TCP socket closed");
    this.connected = false;
  }

  onClose() {
    if (!this.closing) {
      this.closing = true;
      // Just to be sure
      this.close();
      this.emit('close');
    }
  }
}

let tunnelCounter = 0;

function onTunnelClose() {
  tunnelCounter++;
  console.log("Opening tunnel " + tunnelCounter);

  const tunnel = new Tunnel(tunnel_host, "localhost", 8000);
  tunnel.on('close', () => { onTunnelClose(); });
}

// Make a pool
for (let i = 0; i < 10; i++) {
  onTunnelClose();
}
