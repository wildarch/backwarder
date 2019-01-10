const WebSocket = require('ws');
const net = require('net');

const tcpSocket = net.Socket();
let connected = false;
const ws = new WebSocket('ws://localhost:8888');

ws.on('message', data => {
    if(!connected) {
        tcpSocket.connect(8000, "localhost");
        tcpSocket.on('end', () => {
            console.log("Closed!");
            ws.close();
        });
        tcpSocket.on('data', data => {
            ws.send(data);
        });
        connected = true;
    }
    tcpSocket.write(data);
});


