// The billing service. It receives the call from the gateway under the same trace, so its
// span (and any work it does) shows up stitched into the gateway's request in rewynd.
import http from 'node:http';

const server = http.createServer((req, res) => {
  if (req.method === 'POST' && req.url === '/charge') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ id: 'ch_123', status: 'paid' }));
    return;
  }
  res.writeHead(404).end();
});

server.listen(3001, () => console.log('billing listening on :3001'));
