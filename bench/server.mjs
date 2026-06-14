// Minimal server for the shim-overhead benchmark — a trivial endpoint so we measure the
// shim's hot-path cost (instrumentation + async export), not app or DB work.
import express from 'express';

const app = express();
app.use(express.json());
app.get('/ping', (_req, res) => res.json({ ok: true }));

const port = Number(process.env.PORT || 4555);
app.listen(port, () => process.stdout.write('READY\n'));
