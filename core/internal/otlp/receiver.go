// Package otlp is the core's ingest endpoint: an OTLP/HTTP receiver that decodes traces and
// logs and writes them to the store. It is the only thing in rewynd that speaks OTLP on the wire.
package otlp

import (
	"compress/gzip"
	"io"
	"net/http"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/SrinjoyDev/rewynd/core/internal/ingest"
	"github.com/SrinjoyDev/rewynd/core/internal/store"
)

type Receiver struct {
	store *store.Store
}

func NewReceiver(s *store.Store) *Receiver { return &Receiver{store: s} }

func (rc *Receiver) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/traces", rc.handleTraces)
	mux.HandleFunc("POST /v1/logs", rc.handleLogs)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	return mux
}

func (rc *Receiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	body, err := readBody(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var msg coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(body, &msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := rc.store.WriteBatch(ingest.DecodeTraces(&msg)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeProto(w, &coltracepb.ExportTraceServiceResponse{})
}

func (rc *Receiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	body, err := readBody(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var msg collogspb.ExportLogsServiceRequest
	if err := proto.Unmarshal(body, &msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := rc.store.WriteBatch(ingest.DecodeLogs(&msg)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeProto(w, &collogspb.ExportLogsServiceResponse{})
}

func readBody(req *http.Request) ([]byte, error) {
	defer req.Body.Close()
	var r io.Reader = req.Body
	if req.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return io.ReadAll(r)
}

func writeProto(w http.ResponseWriter, m proto.Message) {
	b, err := proto.Marshal(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	_, _ = w.Write(b)
}
