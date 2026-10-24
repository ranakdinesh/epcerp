package jobs

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/logging"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/application"
	"time"
)

// Runtime owns the worker lifecycle; no provider call occurs inside a user transaction.
func RunCredentialMail(ctx context.Context, s *application.CredentialDeliveryService, logger logging.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		for range 10 {
			if ctx.Err() != nil {
				return
			}
			worked, e := s.Deliver(ctx)
			if e != nil {
				logger.Error().Str("event", "auth.credential_email.worker_failed").Msg("Credential email delivery needs attention")
				break
			}
			if !worked {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
