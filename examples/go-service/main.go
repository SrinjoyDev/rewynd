// A Go HTTP service recorded by rewynd. Start() wires OpenTelemetry to the local core; otelhttp
// records the request, and a child span tagged as a DB query shows up correlated under it.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	rewynd "github.com/SrinjoyDev/rewynd/sdk/go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	ctx := context.Background()
	shutdown, err := rewynd.Start(ctx, rewynd.WithService("widgets"))
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(context.Background())

	tracer := otel.Tracer("widgets")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/widgets", func(w http.ResponseWriter, r *http.Request) {
		// A child span that looks like a DB query — rewynd records it as one, correlated to
		// this request. In a real app this is otelsql doing it for you.
		_, span := tracer.Start(r.Context(), "query", trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.statement", "SELECT id, name FROM widgets WHERE active = $1"),
		))
		time.Sleep(15 * time.Millisecond)
		span.End()

		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"name":"sprocket"}]`))
	})

	handler := otelhttp.NewHandler(mux, "http.server")
	log.Println("widgets listening on :8090")
	log.Fatal(http.ListenAndServe(":8090", handler))
}
