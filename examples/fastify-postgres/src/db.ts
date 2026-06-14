import pg from 'pg';

const { Pool } = pg;

export const pool = new Pool({
  connectionString: process.env.DATABASE_URL ?? 'postgresql://rewynd:rewynd@localhost:5433/app',
});

export async function initDb(): Promise<void> {
  for (let i = 0; i < 40; i++) {
    try {
      await pool.query('SELECT 1');
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 500));
    }
  }
  await pool.query(`
    CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS posts (id SERIAL PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT NOT NULL);
    CREATE TABLE IF NOT EXISTS orders (id SERIAL PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), total NUMERIC NOT NULL);
  `);
  const { rows } = await pool.query('SELECT COUNT(*)::int AS n FROM users');
  if (rows[0].n === 0) {
    for (let u = 1; u <= 10; u++) await pool.query('INSERT INTO users (name) VALUES ($1)', [`user${u}`]);
    for (let p = 1; p <= 30; p++) {
      await pool.query('INSERT INTO posts (user_id, title) VALUES ($1, $2)', [((p - 1) % 10) + 1, `post ${p}`]);
    }
  }
}
