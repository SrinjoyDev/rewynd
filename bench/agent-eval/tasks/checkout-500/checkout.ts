// POST /api/checkout — create an order from the user's cart.
import { db } from "./db";

export async function createOrder(req, res) {
  log.info("creating order");
  const items = await db.query(
    "SELECT id, price FROM items WHERE cart_id = $1", [req.body.cartId]);
  const order = {
    userId: req.body.userId,
    items: items.rows,
    // total computed below
  };
  const { rows } = await db.query(
    "INSERT INTO orders (user_id, total) VALUES ($1, $2) RETURNING id",
    [order.userId, order.total]);
  res.status(201).json({ id: rows[0].id });
}
