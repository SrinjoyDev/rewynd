import { rm } from 'node:fs/promises';
import EmbeddedPostgres from 'embedded-postgres';
import pg from 'pg';

const { Pool } = pg;

/** Shared connection pool. Populated by initDb(). */
export let pool: pg.Pool;

let embedded: EmbeddedPostgres | null = null;

/**
 * Boot a real, in-process Postgres (no Docker, no system install). The real `pg`
 * driver connects over TCP, so OpenTelemetry's `pg` instrumentation patches it
 * authentically — exactly what we need to validate zero-config capture.
 */
async function bootEmbedded(): Promise<string> {
  const dir = new URL('../.pgdata', import.meta.url).pathname;
  await rm(dir, { recursive: true, force: true });
  embedded = new EmbeddedPostgres({
    databaseDir: dir,
    user: 'rewynd',
    password: 'rewynd',
    port: 5433,
    persistent: false,
  });
  await embedded.initialise();
  await embedded.start();
  try {
    await embedded.createDatabase('app');
  } catch {
    /* already exists */
  }
  return 'postgresql://rewynd:rewynd@localhost:5433/app';
}

export async function initDb(): Promise<void> {
  // Allow an external Postgres (e.g. Docker) via DATABASE_URL; otherwise embed one.
  const url = process.env.DATABASE_URL ?? (await bootEmbedded());
  pool = new Pool({ connectionString: url });
  await waitForConnection();
  await migrateAndSeed();
}

/** Retry the first connection so a still-booting Postgres (e.g. a fresh container) is fine. */
async function waitForConnection(tries = 40): Promise<void> {
  for (let i = 0; i < tries; i++) {
    try {
      await pool.query('SELECT 1');
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 500));
    }
  }
  throw new Error('database not reachable after retries');
}

async function migrateAndSeed(): Promise<void> {
  await pool.query(`
    CREATE TABLE IF NOT EXISTS users (
      id SERIAL PRIMARY KEY,
      name TEXT NOT NULL
    );
    CREATE TABLE IF NOT EXISTS posts (
      id SERIAL PRIMARY KEY,
      user_id INTEGER NOT NULL REFERENCES users(id),
      title TEXT NOT NULL
    );
    CREATE TABLE IF NOT EXISTS orders (
      id SERIAL PRIMARY KEY,
      user_id INTEGER NOT NULL REFERENCES users(id),
      total NUMERIC NOT NULL
    );
  `);
  const { rows } = await pool.query('SELECT COUNT(*)::int AS n FROM users');
  if (rows[0].n === 0) {
    for (let u = 1; u <= 10; u++) {
      await pool.query('INSERT INTO users (name) VALUES ($1)', [`user${u}`]);
    }
    for (let p = 1; p <= 30; p++) {
      await pool.query('INSERT INTO posts (user_id, title) VALUES ($1, $2)', [
        ((p - 1) % 10) + 1,
        `post ${p}`,
      ]);
    }
  }
}

export async function stopDb(): Promise<void> {
  await pool?.end();
  await embedded?.stop();
}
