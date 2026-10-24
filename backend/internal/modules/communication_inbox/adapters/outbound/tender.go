package outbound

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/application"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/communication_inbox/ports"
	events "github.com/ranakdinesh/epcERP/internal/modules/event_delivery/contracts"
	tender "github.com/ranakdinesh/epcERP/internal/modules/tender_fms/contracts"
)

type tenderSource struct{ source tender.AssignmentActions }

func (s tenderSource) Read(ctx context.Context, id string) (domain.TenderAction, error) {
	a, e := s.source.ReadAction(ctx, id)
	if e != nil {
		return domain.TenderAction{}, e
	}
	return domain.TenderAction{ID: a.ID, AssignmentID: a.AssignmentID, RecipientID: a.RecipientID, EventID: a.EventID, TenderID: a.TenderID, ActorID: a.ActorID, Version: a.AssignmentVersion, At: a.At}, nil
}

type tenderWriter struct {
	c       database.Connection
	r       ports.Repository
	sources ports.TenderSources
	values  ports.Values
}

func NewTenderWriter(c database.Connection, j events.Writer, s tender.AssignmentActions) contracts.TenderWriter {
	return &tenderWriter{c, &repository{db.New(c), j}, tenderSource{s}, Values{}}
}
func (w *tenderWriter) TenderAssignment(ctx context.Context, action string) (string, error) {
	if !database.IsTransactionConnection(ctx, w.c) {
		return "", errors.New("Tender notice requires the source transaction")
	}
	return application.DeliverTender(ctx, w.r, w.values, w.sources, action)
}
