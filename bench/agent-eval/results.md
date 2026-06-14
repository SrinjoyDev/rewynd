# Results

A coding agent (Claude, via the same harness in both conditions) was asked to find the root
cause of four backend bugs. In the **without** condition it had the source code and the
application logs — what you have today. In the **with** condition it additionally had rewynd
and could inspect the recorded trace. Same model, same bug, same prompt; the only difference is
rewynd.

| Bug | Symptom in the logs | Without rewynd | With rewynd |
|---|---|---|---|
| `POST /api/checkout` → 500 | `request failed` | ✅ correct (high) | ✅ correct (high) |
| `GET /api/users` slow | nothing useful | ✅ correct (high) | ✅ correct (high) |
| `POST /api/pay` → 502 | `charging` | ⚠️ correct but **unsure** (medium — couldn't confirm the upstream actually failed) | ✅ correct (high — saw the outbound 500) |
| `email.send` job failed | nothing (silent) | ❌ **wrong** — guessed a job-type mismatch | ✅ correct (high — read the SMTP exception) |

**Correct root cause: 4/4 with rewynd, 3/4 without. Correct *and* confident: 4/4 vs 2/4.**

## What the numbers actually say

The two bugs whose cause is visible in the source — the missing `total` and the N+1 loop — both
agents solved either way. rewynd's value showed up exactly where it should: on the failures
whose cause **isn't in the code you're staring at.**

- **`pay-502`:** without rewynd the agent reasoned its way to "the upstream probably failed,"
  but hedged to *medium* confidence — it couldn't tell a real upstream 500 from a timeout or a
  local throw. With rewynd it saw `POST https://payments.acme.com/v1/charge → 500` and was
  certain.
- **`email-job`:** the important one. A queue job failed silently — the logs show only "worker
  started." Without rewynd the agent produced a **confident, plausible, wrong** diagnosis: a
  job-type string mismatch. A developer (or an agent) acting on that would "fix" the type
  matching and ship nothing. With rewynd it read the recorded exception —
  `SMTPConnectError: connection refused` — and got it right.

That last case is the whole thesis: without ground truth, agents don't just fail — they
**hallucinate a fix.** rewynd replaces the guess with what actually happened.

## Caveats (read these)

- **n = 4.** This is a small, illustrative suite, not a statistical claim. It's fully
  reproducible — run it yourself (see [README](./README.md)) and add your own bugs.
- The bugs are realistic but hand-built. The point isn't a precise percentage; it's the
  *shape*: rewynd flips the invisible-cause failures from guess/wrong to certain.
- Same agent and prompt in both arms; the only variable is rewynd.
