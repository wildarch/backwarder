const express = require('express');
const app = express();
const expressWs = require('express-ws')(app);

let freeSockets = [];

app.ws('/', (ws, req) => {
    freeSockets.push(ws);
    console.log("New backconnect socket added")
});

app.get('/', (req, res, next) => {
    const ws = freeSockets.pop();
    if(!ws) {
        res.status(503);
        res.end();
        return;
    }

    ws.send(Buffer.from("GET " + req.originalUrl + " HTTP/1.1\r\n"));
    for(let header in req.headers) {
        if(req.headers.hasOwnProperty(header)) {
            ws.send(Buffer.from(header + ": " + req.headers[header] + "\r\n"));
        }
    }
    ws.send(Buffer.from("\r\n"));

    req.on('data', chunk => {
        ws.send(chunk);
    });
    ws.on('message', chunk => {
        res.write(chunk);
    });
    ws.on('close', () => {
        res.end();
    });
    req.on('close', () => {
        ws.terminate();
    });
});

app.listen(8888);