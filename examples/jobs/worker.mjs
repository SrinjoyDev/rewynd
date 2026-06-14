// A background worker draining an in-memory queue. Each job runs inside a CONSUMER span — no
// HTTP request involved — so rewynd records it as a first-class "job" flow, with the work it
// did (here, an outbound call to the sink) correlated under it. One job fails on purpose.
import { trace, SpanKind, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('jobs-example');
const SINK = process.env.SINK_URL ?? 'http://localhost:3002';

const queue = [
  { id: 1, type: 'orders.created', amount: 4200 },
  { id: 2, type: 'orders.created', amount: 9900 },
  { id: 3, type: 'orders.created', amount: -1 }, // the sink rejects this → the job fails
];

async function processJob(job) {
  await tracer.startActiveSpan(
    `${job.type} process`,
    {
      kind: SpanKind.CONSUMER,
      attributes: {
        'messaging.system': 'memory',
        'messaging.destination.name': job.type,
        'messaging.operation': 'process',
        'messaging.message.id': String(job.id),
      },
    },
    async (span) => {
      try {
        const r = await fetch(`${SINK}/charge`, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ amount: job.amount }),
        });
        if (!r.ok) throw new Error(`charge failed: ${r.status}`);
        span.setStatus({ code: SpanStatusCode.OK });
        console.log(`job ${job.id} ok`);
      } catch (err) {
        span.recordException(err);
        span.setStatus({ code: SpanStatusCode.ERROR, message: err.message });
        console.log(`job ${job.id} failed: ${err.message}`);
      } finally {
        span.end();
      }
    },
  );
}

for (const job of queue) {
  await processJob(job);
}
console.log('drained the queue');
// No manual flush needed: rewynd's shim flushes recorded spans when the process exits.
