const net = require('net');
const WebSocket = require('ws');
const EventEmitter = require('events');
const HTTPParser = process.binding('http_parser').HTTPParser;

const port = process.env.PORT || 8888;

class WebsocketRequestParser extends EventEmitter {
  constructor() {
    super();
    this.first_packet = true;
    this.parser = new HTTPParser(HTTPParser.REQUEST);
  }

  consume(request_bytes) {
    const kOnHeadersComplete = HTTPParser.kOnHeadersComplete | 0;
    this.parser[kOnHeadersComplete] =
        (versionMajor, versionMinor, headers, method, url, statusCode,
         statusMessage, upgrade, shouldKeepAlive) => {
          let req = {
            method : method === 1 ? 'GET' : null,
            headers : {},
          };
          for (let i = 0; i < headers.length; i += 2) {
            req.headers[headers[i].toLowerCase()] = headers[i + 1];
          }
          if (req.headers.upgrade &&
              req.headers.upgrade.toLowerCase() === 'websocket' &&
	      url.endsWith("/backwarder")) {
            // Websocket request
            this.emit("websocket", req, request_bytes);
          } else {
            // Regular http
            this.isRegular = true;
            this.emit("http", req, request_bytes);
          }
        };
    this.parser.execute(request_bytes);
  }
}

let freeSockets = [];
let socketCounter = 0;

const server = new net.Server();
const wss = new WebSocket.Server({noServer : true});

server.on('connection', socket => {
  const parser = new WebsocketRequestParser();
  parser.on('websocket', (req, data) => {
    wss.handleUpgrade(req, socket, data, ws => {
      console.log("Registered new websocket");
      freeSockets.push(ws);
    });
  });
  let webSocket = null;
  parser.on('http', (req, data) => {
    // Assign a web socket
    webSocket = freeSockets.pop();
    if (!webSocket) {
      console.warn("No websockets are available, rejecting connection");
      socket.end();
      return;
    }
    webSocket.on('message', data => { socket.write(data); });
    webSocket.on('close', () => {
      console.log("Closed webSocket connection");
      socket.end();
    });
    socket.on('end', () => {
      console.log("Closed TCP connection");
      webSocket.terminate();
    });

    // Send header data
    webSocket.send(data);
  });
  let first_packet = true;
  socket.on('data', data => {
    if (first_packet) {
      first_packet = false;
      parser.consume(data);
    } else if (webSocket) {
      webSocket.send(data);
    }
  });
});

server.listen(port);
