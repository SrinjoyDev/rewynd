// rewynd capture shim: loaded via `node --import` before app code. Configures OpenTelemetry
// auto-instrumentation + log correlation and exports to the local rewynd core over OTLP.

import { register as registerLoaderHook } from 'node:module';

if (process.env.NODE_ENV === 'production' && process.env.REWYND_FORCE !== '1') {
  process.stderr.write(
    '[rewynd] refusing to start under NODE_ENV=production (set REWYND_FORCE=1 to override)\n',
  );
} else {
  // The ESM hook must be installed before instrumented libs are imported (ESM, not just CJS).
  try {
    registerLoaderHook('@opentelemetry/instrumentation/hook.mjs', import.meta.url);
  } catch (err) {
    process.stderr.write(`[rewynd] ESM hook unavailable: ${err?.message}\n`);
  }

  const { NodeSDK } = await import('@opentelemetry/sdk-node');
  const { getNodeAutoInstrumentations } = await import('@opentelemetry/auto-instrumentations-node');
  const { OTLPTraceExporter } = await import('@opentelemetry/exporter-trace-otlp-proto');
  const { OTLPLogExporter } = await import('@opentelemetry/exporter-logs-otlp-proto');
  const { BatchLogRecordProcessor } = await import('@opentelemetry/sdk-logs');

  // Optional, version-safe instrumentations the user opted into. Prisma isn't in the auto
  // set; Prisma users add `@prisma/instrumentation` (matching their Prisma) and we load it.
  const extra = [];
  for (const [pkg, exportName] of [['@prisma/instrumentation', 'PrismaInstrumentation']]) {
    try {
      const mod = await import(pkg);
      if (mod[exportName]) extra.push(new mod[exportName]());
    } catch {}
  }

  // Headers are redacted here, inside your app — secrets never reach the core.
  const REDACT = new Set(['authorization', 'cookie', 'set-cookie', 'proxy-authorization', 'x-api-key', 'api-key']);
  const redactHeaders = (h) => {
    const out = {};
    for (const [k, v] of Object.entries(h ?? {})) {
      const key = k.toLowerCase();
      out[key] = REDACT.has(key) ? '[redacted]' : Array.isArray(v) ? v.join(', ') : String(v);
    }
    return out;
  };

  // Body capture is read-only and post-parse (res.req.body), so it never interferes with the
  // app's own body reading. Secrets are redacted; large bodies are size-capped.
  const BODY_REDACT = new Set([
    ...REDACT, 'password', 'pwd', 'token', 'secret', 'api_key', 'apikey', 'access_token', 'refresh_token',
  ]);
  const redactBody = (v, depth = 0) => {
    if (v == null || depth > 6) return v;
    if (Array.isArray(v)) return v.map((x) => redactBody(x, depth + 1));
    if (typeof v === 'object') {
      const out = {};
      for (const [k, val] of Object.entries(v)) {
        out[k] = BODY_REDACT.has(k.toLowerCase()) ? '[redacted]' : redactBody(val, depth + 1);
      }
      return out;
    }
    return v;
  };
  const captureBody = (body) => {
    if (body == null) return undefined;
    if (typeof body === 'object' && !Array.isArray(body) && Object.keys(body).length === 0) return undefined;
    let s;
    try {
      s = JSON.stringify(redactBody(body));
    } catch {
      return undefined;
    }
    const cap = 16 * 1024;
    return s.length > cap ? s.slice(0, cap) + '...[truncated]' : s;
  };

  // Exporters default to http://localhost:4318 — the core's OTLP endpoint. Zero config.
  const sdk = new NodeSDK({
    serviceName: process.env.REWYND_SERVICE ?? process.env.npm_package_name ?? 'app',
    traceExporter: new OTLPTraceExporter(),
    logRecordProcessors: [new BatchLogRecordProcessor(new OTLPLogExporter())],
    instrumentations: [
      getNodeAutoInstrumentations({
        '@opentelemetry/instrumentation-fs': { enabled: false },
        '@opentelemetry/instrumentation-dns': { enabled: false },
        '@opentelemetry/instrumentation-net': { enabled: false },
        '@opentelemetry/instrumentation-http': {
          requestHook: (span, req) => {
            try {
              if (req?.headers) span.setAttribute('rewynd.request.headers', JSON.stringify(redactHeaders(req.headers)));
            } catch {}
          },
          responseHook: (span, res) => {
            try {
              const h = typeof res?.getHeaders === 'function' ? res.getHeaders() : res?.headers;
              if (h) span.setAttribute('rewynd.response.headers', JSON.stringify(redactHeaders(h)));
            } catch {}
          },
          applyCustomAttributesOnSpan: (span, request) => {
            try {
              const body = captureBody(request?.body); // parsed by the time the span ends
              if (body) span.setAttribute('rewynd.request.body', body);
            } catch {}
          },
        },
      }),
      ...extra,
    ],
  });
  sdk.start();

  const shutdown = () => sdk.shutdown().finally(() => process.exit(0));
  process.on('SIGTERM', shutdown);
  process.on('SIGINT', shutdown);
}
