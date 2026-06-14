import Fastify from 'fastify';
import { initDb, pool } from './db.js';

const app = Fastify({ logger: true });
const port = Number(process.env.PORT ?? 3001);

app.get('/api/feed', async () => {
  app.log.info('fetching feed');
  const { rows } = await pool.query('SELECT id, title FROM posts ORDER BY id DESC LIMIT 10');
  return rows;
});

// BUG 1 — N+1: list users, then one SELECT per user for their posts.
app.get('/api/users', async () => {
  app.log.info('listing users');
  const { rows: users } = await pool.query('SELECT id, name FROM users ORDER BY id');
  const out: unknown[] = [];
  for (const u of users) {
    const { rows: posts } = await pool.query('SELECT id, title FROM posts WHERE user_id = $1', [u.id]);
    out.push({ ...u, posts });
  }
  app.log.info({ users: users.length }, 'assembled users with posts');
  return out;
});

// BUG 2 — contextual 500: NOT NULL violation on orders.total.
app.post('/api/orders', async (req, reply) => {
  const userId = Number((req.body as { userId?: number })?.userId ?? 1);
  app.log.info({ userId }, 'creating order');
  const { rows } = await pool.query(
    'INSERT INTO orders (user_id, total) VALUES ($1, $2) RETURNING id',
    [userId, null],
  );
  return reply.code(201).send(rows[0]);
});

// Outbound HTTP — captured as an outbound call (undici/http instrumentation).
app.get('/api/proxy', async () => {
  const res = await fetch(`http://localhost:${port}/api/feed`);
  return { proxied: await res.json() };
});

await initDb();
await app.listen({ port, host: '127.0.0.1' });
