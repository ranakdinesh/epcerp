package contracts

import "github.com/ranakdinesh/epcERP/internal/modules/document_reading/domain"

// Published pure reading contract; no consuming module imports reader adapters.
type Result = domain.Result
type Evidence = domain.Evidence
type EvidenceBlock = domain.EvidenceBlock

func NormalizeEvidence(result Result) Evidence { return domain.BuildEvidence(result) }
