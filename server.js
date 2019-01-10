const express = require('express');
const app = express();
const expressWs = require('express-ws')(app);

const port = process.env.PORT || 8888;

let freeSockets = [];

let socketCounter = 0;

app.ws('/', (ws, req) => {
    freeSockets.push(ws);
    socketCounter++;
    console.log("Added backconnect socket " + socketCounter);
});

app.use((req, res) => {
    const ws = freeSockets.pop();

    // If no free backconnect socket is available, report temporarily unavailable.
    if(!ws) {
        res.status(503);
        res.end();
        return;
    }

    // Reconstruct HTTP Header
    ws.send(Buffer.from(req.method + " " + req.originalUrl + " HTTP/1.1\r\n"));
    for(let header in req.headers) {
        if(req.headers.hasOwnProperty(header)) {
            ws.send(Buffer.from(header + ": " + req.headers[header] + "\r\n"));
        }
    }
    ws.send(Buffer.from("\r\n"));

    // Forward data
    req.on('data', chunk => {
        ws.send(chunk);
    });
    ws.on('message', chunk => {
        res.write(chunk);
    });

    // Synchronize close
    ws.on('close', () => {
        res.end();
    });
    req.on('close', () => {
        ws.terminate();
    });
});

app.listen(port);
