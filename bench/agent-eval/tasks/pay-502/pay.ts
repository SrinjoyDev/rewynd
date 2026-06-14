// POST /api/pay — charge the user via the payments provider.
export async function pay(req, res) {
  log.info("charging");
  const r = await fetch("https://payments.acme.com/v1/charge", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ amount: req.body.amount, token: req.body.token }),
  });
  if (!r.ok) {
    return res.status(502).json({ error: "payment failed" });
  }
  res.json(await r.json());
}
