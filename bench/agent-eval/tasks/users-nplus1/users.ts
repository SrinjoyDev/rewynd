// GET /api/users — list users with their posts. (Latency has been creeping up.)
import { db } from "./db";

export async function listUsers(req, res) {
  log.info("listing users");
  const users = await db.query("SELECT id, name FROM users ORDER BY id");
  const out = [];
  for (const u of users.rows) {
    const posts = await db.query(
      "SELECT id, title FROM posts WHERE user_id = $1", [u.id]);
    out.push({ ...u, posts: posts.rows });
  }
  res.json(out);
}
