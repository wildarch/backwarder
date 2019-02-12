const WebSocket = require('ws');
const net = require('net');
const EventEmitter = require('events');
const schedule = require('node-schedule');

function main() {
  let tunnel_host = process.argv[2];
  let target_port = process.argv[3];

  const manager = new TunnelManager(tunnel_host, "localhost", target_port, 10);
  manager.start();
  console.log("Ready to rock and roll");

  const startRule = new schedule.RecurrenceRule();
  startRule.hour = 8;
  schedule.scheduleJob(startRule, () => {
    console.log("Firing up clients");
    manager.start()
  });

  const endRule = new schedule.RecurrenceRule();
  endRule.hour = 22;
  schedule.scheduleJob(endRule, () => {
    console.log("Shutting down for the night");
    manager.shutDown()
  });
}

class Tunnel extends EventEmitter {
  constructor(id, socketUrl, tcpHost, tcpPort) {
    super();
    this.id = id;
    this.tcpSocket = new net.Socket();
    this.connected = false;
    this.closing = false;
    this.webSocket = new WebSocket(socketUrl);
    this.created = new Date();

    this.webSocket.on('message', data => {
      if (!this.connected) {
        this.tcpSocket.connect(tcpPort, tcpHost);
        this.tcpSocket.on('end', () => {
          console.log("Tunnel " + this.id + ":TCP socket end");
          this.onClose();
        });
        this.tcpSocket.on('data', data => { this.webSocket.send(data); });
        this.connected = true;
      }
      this.tcpSocket.write(data);
    });

    this.webSocket.on('close', () => {
      console.log("Tunnel " + this.id + ":websocket end");
      this.onClose();
    });

    this.webSocket.on('error', (e) => {
      console.error("Websocket error: ", e);
      this.emit('error', e);
      this.onClose();
    });
  }

  close() {
    this.tcpSocket.end();
    this.webSocket.close(1000, "TCP socket closed");
    console.log("Tunnel " + this.id + " has closed");
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

class TunnelManager {
  constructor(tunnel_url, target_host, target_port, tunnel_count) {
    this.tunnel_url = tunnel_url;
    this.target_host = target_host;
    this.target_port = target_port;
    this.target_tunnel_count = tunnel_count;

    this.total_tunnel_count = 0;
    this.shutting_down = false;
    this.active_tunnels = [];
    this.tunnel_connect_errors = 0;
  }

  start() {
    this.shutting_down = false;
    this.tunnel_connect_errors = 0;
    for (let i = 0; i < this.target_tunnel_count; i++) {
      this.addTunnel();
    }
  }

  addTunnel() {
    if (this.shutting_down) {
      console.warn(
          "Not starting another tunnel because we are already shutting down");
      return;
    }
    this.total_tunnel_count++;
    // Each tunnel gets its own id.
    const id = this.total_tunnel_count;
    const tunnel =
        new Tunnel(id, this.tunnel_url, this.target_host, this.target_port);

    tunnel.on('close', () => this.onTunnelClose(tunnel));
    tunnel.on('error', (e) => this.onTunnelError(e, tunnel));
    this.active_tunnels.push(tunnel);
  }

  onTunnelClose(tunnel) { 
    let idx = this.active_tunnels.indexOf(tunnel);
    this.active_tunnels.splice(idx, 1);
    this.addTunnel(); 
  }

  onTunnelError(err, tunnel) {
    if (Date.now() - tunnel.created < 5000) {
      console.error("Tunnel " + tunnel.id + " died shortly after creation: ",
                    err);
      this.tunnel_connect_errors++;
      if (this.tunnel_connect_errors > 10) {
        console.error(
            "More than 10 tunnel died in short period of time, shutting down");
        this.shutDown();
      }
    } else {
      console.warn("Tunnel " + tunnel.id + " encountered an error: ", err);
    }
  }

  shutDown() {
    this.shutting_down = true;
    console.log("Shutdown has started");
    for (let tunnel of this.active_tunnels) {
      tunnel.close();
    }
    this.active_tunnels = [];
    console.log("All tunnels closed");
  }
}

main();
