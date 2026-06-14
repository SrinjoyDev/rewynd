"""FastAPI + Postgres example — captured by the same rewynd core as the Node apps, over OTLP."""

import logging
import os

import psycopg2
from fastapi import FastAPI

logging.basicConfig(level=logging.INFO, format="%(message)s")
log = logging.getLogger("app")

DSN = os.environ.get("DATABASE_URL", "postgresql://rewynd:rewynd@localhost:5433/app")
app = FastAPI()


def db():
    return psycopg2.connect(DSN)


@app.get("/api/feed")
def feed():
    log.info("fetching feed")
    with db() as conn, conn.cursor() as cur:
        cur.execute("SELECT id, title FROM posts ORDER BY id DESC LIMIT 10")
        return [{"id": r[0], "title": r[1]} for r in cur.fetchall()]


# BUG 1 — N+1: list users, then one SELECT per user for their posts.
@app.get("/api/users")
def users():
    log.info("listing users")
    out = []
    with db() as conn:
        with conn.cursor() as cur:
            cur.execute("SELECT id, name FROM users ORDER BY id")
            rows = cur.fetchall()
        for uid, name in rows:
            with conn.cursor() as c2:
                c2.execute("SELECT id, title FROM posts WHERE user_id = %s", (uid,))
                posts = [{"id": r[0], "title": r[1]} for r in c2.fetchall()]
            out.append({"id": uid, "name": name, "posts": posts})
    log.info("assembled users with posts: %d", len(out))
    return out


# BUG 2 — contextual 500: NOT NULL violation on orders.total.
@app.post("/api/orders")
def create_order():
    log.info("creating order")
    with db() as conn, conn.cursor() as cur:
        cur.execute("INSERT INTO orders (user_id, total) VALUES (%s, %s) RETURNING id", (1, None))
        return {"id": cur.fetchone()[0]}
