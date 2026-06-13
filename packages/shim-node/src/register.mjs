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
      }),
    ],
  });
  sdk.start();

  const shutdown = () => sdk.shutdown().finally(() => process.exit(0));
  process.on('SIGTERM', shutdown);
  process.on('SIGINT', shutdown);
}
