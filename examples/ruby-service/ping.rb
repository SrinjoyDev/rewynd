# A Ruby service recorded by rewynd with NO rewynd-specific code — just standard
# OpenTelemetry. The OTLP exporter reads OTEL_EXPORTER_OTLP_ENDPOINT, which `rewynd run` sets,
# so this is the "any OpenTelemetry language" path that also covers Java, .NET, PHP, and more.
require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'

OpenTelemetry::SDK.configure do |c|
  c.service_name = 'ruby-demo'
end

tracer = OpenTelemetry.tracer_provider.tracer('demo')
tracer.in_span('GET /api/ping', kind: :server, attributes: {
  'http.request.method' => 'GET',
  'url.path' => '/api/ping',
  'http.response.status_code' => 200
}) do
  # A child span that looks like a DB query — rewynd records it correlated under the request.
  tracer.in_span('query', kind: :client, attributes: {
    'db.system' => 'postgresql',
    'db.statement' => 'SELECT id, email FROM accounts WHERE id = $1'
  }) { sleep 0.01 }
end

OpenTelemetry.tracer_provider.shutdown
puts 'ruby: sent a trace to rewynd'
