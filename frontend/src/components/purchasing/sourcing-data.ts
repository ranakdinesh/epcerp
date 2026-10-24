export type SourcingOffer = {
  id: string;
  supplier_id: string;
  supplier_snapshot: {
    name: string;
    code: string;
    registration: string;
    contact: string;
    location: string;
  };
  state: string;
  due_date: string;
  rfq_terms: string;
  sent_date: string | null;
  channel: string;
  dispatch_reference: string;
  quote_reference: string;
  evidence_reference: string;
  valid_until: string | null;
  unit_price: string;
  discount: string;
  tax: string;
  freight: string;
  other: string;
  delivery_days: number;
  payment_terms: string;
  warranty: string;
  compliant: boolean;
  compliance_notes: string;
  no_bid_reason: string;
};
export type SourcingCase = {
  id: string;
  version: number;
  state: string;
  approver_id: string;
  selected_offer_id: string | null;
  selection_reason: string;
  loi_terms: string;
  approved_by_id: string | null;
  approved_at: string | null;
  proposed_by_id: string | null;
  loi_reference: string;
  loi_sent_date: string | null;
  loi_channel: string;
  loi_dispatch_reference: string;
  source_snapshot: Record<string, string>;
  decision_snapshot: {
    source?: Record<string, string>;
    offer?: SourcingOffer;
    terms?: string;
    reason?: string;
    comparison?: { offer_id: string; total: string }[];
  };
  offers: SourcingOffer[];
};
export type SourcingWorkspace = {
  case: SourcingCase | null;
  source: Record<string, string>;
  actions: string[];
  suppliers: { id: string; name: string; code: string; eligible: boolean }[];
  approvers: { id: string; label: string }[];
  comparison: {
    offer_id: string;
    total: string;
    variance: string;
    rank: number | null;
    expired: boolean;
  }[];
  history: {
    id: string;
    action: string;
    reason: string;
    created_at: string;
    created_by_id: string | null;
  }[];
};
