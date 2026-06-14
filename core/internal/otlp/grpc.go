package otlp

import (
	"context"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"

	"github.com/SrinjoyDev/rewynd/internal/ingest"
	"github.com/SrinjoyDev/rewynd/internal/store"
)

// NewGRPCServer builds an OTLP/gRPC server over the same store and decode pipeline as the
// HTTP receiver. Most OpenTelemetry SDKs default to gRPC on :4317, so accepting it lets any
// instrumented service feed rewynd without reconfiguration.
func NewGRPCServer(s *store.Store) *grpc.Server {
	gs := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(gs, &traceService{store: s})
	collogspb.RegisterLogsServiceServer(gs, &logsService{store: s})
	return gs
}

// The two OTLP services each declare a method named Export, so they need separate receivers.
type traceService struct {
	coltracepb.UnimplementedTraceServiceServer
	store *store.Store
}

func (t *traceService) Export(_ context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	if err := t.store.WriteBatch(ingest.DecodeTraces(req)); err != nil {
		return nil, err
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type logsService struct {
	collogspb.UnimplementedLogsServiceServer
	store *store.Store
}

func (l *logsService) Export(_ context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	if err := l.store.WriteBatch(ingest.DecodeLogs(req)); err != nil {
		return nil, err
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}
