// The gateway service. /api/checkout calls the billing service over HTTP; OpenTelemetry
// propagates the trace across that fetch, so rewynd stitches both services into one trace.
import http from 'node:http';

const BILLING = process.env.BILLING_URL ?? 'http://localhost:3001';

const server = http.createServer(async (req, res) => {
  if (req.method === 'POST' && req.url === '/api/checkout') {
    const r = await fetch(`${BILLING}/charge`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ amount: 4200 }),
    });
    const charge = await r.json();
    res.writeHead(r.ok ? 200 : 502, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ ok: r.ok, charge }));
    return;
  }
  res.writeHead(404).end();
});

server.listen(3000, () => console.log('gateway listening on :3000'));
