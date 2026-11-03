package inbound

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"net/http"
	"time"
)

// Research may count and inspect PDFs for up to 75 seconds. Keep that bounded
// operation inside an explicit request budget without relaxing global HTTP limits.
func boundedTSCResearch(w http.ResponseWriter, r *http.Request) (*http.Request, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(95 * time.Second))
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		cancel()
		return r, func() {}, domain.CaptureFailure(503, "research_transport_unavailable", "Question research is temporarily unavailable")
	}
	return r.WithContext(ctx), cancel, nil
}
