package infrastructure

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure/httpmiddleware"
	"time"
)

// OperationLimits composes bounded per-actor transport quotas with global/IP limits.
// Replicated deployments must enforce the same policy at their reviewed edge.
type OperationLimits struct {
	limits map[string]*httpmiddleware.Limiter
}

func NewReceiptLimits() *OperationLimits {
	return &OperationLimits{map[string]*httpmiddleware.Limiter{"create": httpmiddleware.NewLimiter(10, 10*time.Minute), "seal": httpmiddleware.NewLimiter(10, 10*time.Minute), "read": httpmiddleware.NewLimiter(120, time.Minute), "upload": httpmiddleware.NewLimiter(20, time.Minute)}}
}
func NewTenderLimits() *OperationLimits {
	return &OperationLimits{map[string]*httpmiddleware.Limiter{"capture": httpmiddleware.NewLimiter(10, time.Minute), "assignment": httpmiddleware.NewLimiter(20, time.Minute), "read": httpmiddleware.NewLimiter(120, time.Minute)}}
}
func (l *OperationLimits) Allow(actor, operation string) bool {
	limit := l.limits[operation]
	return limit != nil && limit.Allow(actor)
}
