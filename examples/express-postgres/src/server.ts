import express, { type NextFunction, type Request, type Response } from 'express';
import pino from 'pino';
import { initDb, pool, stopDb } from './db.js';

const log = pino();
const app = express();
app.use(express.json());

const wrap =
  (fn: (req: Request, res: Response, next: NextFunction) => Promise<unknown>) =>
  (req: Request, res: Response, next: NextFunction) =>
    fn(req, res, next).catch(next);

// Healthy endpoint.
app.get(
  '/api/feed',
  wrap(async (_req, res) => {
    log.info('fetching feed');
    const { rows } = await pool.query('SELECT id, title FROM posts ORDER BY id DESC LIMIT 10');
    res.json(rows);
  }),
);

// BUG 1 — N+1: list users, then fire one SELECT per user for their posts.
app.get(
  '/api/users',
  wrap(async (_req, res) => {
    log.info('listing users');
    const { rows: users } = await pool.query('SELECT id, name FROM users ORDER BY id');
    const out: unknown[] = [];
    for (const u of users) {
      const { rows: posts } = await pool.query('SELECT id, title FROM posts WHERE user_id = $1', [
        u.id,
      ]);
      out.push({ ...u, posts });
    }
    log.info({ users: users.length }, 'assembled users with posts');
    res.json(out);
  }),
);

// BUG 2 — contextual 500: NOT NULL violation on orders.total throws inside pg.
app.post(
  '/api/orders',
  wrap(async (req, res) => {
    const userId = Number(req.body?.userId ?? 1);
    log.info({ userId }, 'creating order');
    const { rows } = await pool.query(
      'INSERT INTO orders (user_id, total) VALUES ($1, $2) RETURNING id',
      [userId, null], // bug: total is null -> pg throws "null value in column total"
    );
    res.status(201).json(rows[0]);
  }),
);

// eslint-disable-next-line @typescript-eslint/no-unused-vars
app.use((err: any, _req: Request, res: Response, _next: NextFunction) => {
  log.error({ err: err?.message }, 'request failed');
  res.status(500).json({ error: String(err?.message ?? err) });
});

const port = Number(process.env.PORT ?? 3000);

initDb()
  .then(() => {
    const server = app.listen(port, () =>
      log.info(`example app listening on http://localhost:${port}`),
    );
    const shutdown = async () => {
      server.close();
      await stopDb();
      process.exit(0);
    };
    process.on('SIGINT', shutdown);
    process.on('SIGTERM', shutdown);
  })
  .catch((err) => {
    log.error({ err }, 'failed to start');
    process.exit(1);
  });
