// Measure the rewynd shim's added request latency and fail if it exceeds budget (§9A).
// Runs the same trivial server with and without the shim, fires keep-alive requests, and
// compares p50/p99. The shim exports asynchronously, so this captures real hot-path cost.
import { spawn } from 'node:child_process';
import http from 'node:http';
import path from 'node:path';

const ROOT = path.resolve(import.meta.dirname, '..');
const SHIM = `file://${ROOT}/packages/shim-node/src/register.mjs`;
const PORT = Number(process.env.PORT || 4555);
const N = Number(process.env.BENCH_N || 4000);
const WARMUP = 500;

const agent = new http.Agent({ keepAlive: true, maxSockets: 1 });

function startServer(withShim) {
  const env = { ...process.env, PORT: String(PORT) };
  if (withShim) env.NODE_OPTIONS = `--import ${SHIM}`;
  const child = spawn(process.execPath, [path.join(ROOT, 'bench', 'server.mjs')], { env });
  return new Promise((resolve, reject) => {
    child.stdout.on('data', (d) => d.toString().includes('READY') && resolve(child));
    child.on('error', reject);
    setTimeout(() => reject(new Error('server did not start')), 15000);
  });
}

function once() {
  return new Promise((resolve, reject) => {
    const t = process.hrtime.bigint();
    http
      .get({ host: '127.0.0.1', port: PORT, path: '/ping', agent }, (res) => {
        res.resume();
        res.on('end', () => resolve(Number(process.hrtime.bigint() - t) / 1e6));
      })
      .on('error', reject);
  });
}

async function measure(withShim) {
  const child = await startServer(withShim);
  try {
    for (let i = 0; i < WARMUP; i++) await once();
    const lat = [];
    for (let i = 0; i < N; i++) lat.push(await once());
    lat.sort((a, b) => a - b);
    return { p50: lat[Math.floor(N * 0.5)], p99: lat[Math.floor(N * 0.99)] };
  } finally {
    child.kill('SIGKILL');
  }
}

const base = await measure(false);
const shim = await measure(true);
const dP50 = shim.p50 - base.p50;
const dP99 = shim.p99 - base.p99;

console.log(`baseline   p50=${base.p50.toFixed(3)}ms  p99=${base.p99.toFixed(3)}ms`);
console.log(`with shim  p50=${shim.p50.toFixed(3)}ms  p99=${shim.p99.toFixed(3)}ms`);
console.log(`overhead   p50=+${dP50.toFixed(3)}ms  p99=+${dP99.toFixed(3)}ms`);

const BUDGET_P50 = Number(process.env.BUDGET_P50 || 3);
const BUDGET_P99 = Number(process.env.BUDGET_P99 || 15);
if (dP50 > BUDGET_P50 || dP99 > BUDGET_P99) {
  console.error(`FAIL: shim overhead exceeds budget (p50 ≤ ${BUDGET_P50}ms, p99 ≤ ${BUDGET_P99}ms)`);
  process.exit(1);
}
console.log('OK: within budget');
