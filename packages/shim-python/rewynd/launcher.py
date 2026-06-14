"""rewynd-run: launch a Python app with recording on (OpenTelemetry -> the local rewynd core)."""

import os
import sys

# Point OpenTelemetry at the local core (OTLP/HTTP on 4318) with sane defaults. setdefault
# means a user's own OTEL_* env still wins.
_DEFAULTS = {
    "OTEL_TRACES_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318",
    "OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED": "true",
    "OTEL_PYTHON_LOG_CORRELATION": "true",
    "OTEL_SERVICE_NAME": "app",
}


def main() -> None:
    env = os.environ.get("REWYND_ENV", os.environ.get("ENV", ""))
    if env == "production" and os.environ.get("REWYND_FORCE") != "1":
        sys.stderr.write("[rewynd] refusing to start in production (set REWYND_FORCE=1 to override)\n")

    for key, value in _DEFAULTS.items():
        os.environ.setdefault(key, value)

    args = sys.argv[1:]
    if args and args[0] == "--":
        args = args[1:]
    if not args:
        sys.stderr.write("usage: rewynd-run <command> [args...]   e.g. rewynd-run uvicorn app:app\n")
        sys.exit(1)

    os.execvp("opentelemetry-instrument", ["opentelemetry-instrument", *args])
