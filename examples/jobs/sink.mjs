// A tiny service the worker's jobs call, so each job has correlated outbound work. It rejects
// non-positive amounts, which is how one job is made to fail.
import http from 'node:http';

const server = http.createServer((req, res) => {
  if (req.method === 'POST' && req.url === '/charge') {
    let body = '';
    req.on('data', (c) => (body += c));
    req.on('end', () => {
      const amount = Number(JSON.parse(body || '{}').amount ?? 0);
      if (amount <= 0) {
        res.writeHead(422, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: 'amount must be positive' }));
        return;
      }
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ id: 'ch_ok', status: 'paid' }));
    });
    return;
  }
  res.writeHead(404).end();
});

server.listen(3002, () => console.log('sink listening on :3002'));
