package nexus

import (
	"context"
	"net/http"

	"github.com/graydraft/graydraft/pkg/trace"
)

func applyTraceHeaders(req *http.Request, ctx context.Context) {
	id := trace.FromContext(ctx)
	if id == "" {
		return
	}
	req.Header.Set(trace.HeaderTraceID, id)
	req.Header.Set(trace.HeaderRequestID, id)
}
