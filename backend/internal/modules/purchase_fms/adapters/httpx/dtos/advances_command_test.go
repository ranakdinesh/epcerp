package dtos

import (
	"strings"
	"testing"
)

func TestAdvancesAdvanceCommandsRejectCallerMoneyAuthorityOrPaymentFlags(t *testing.T) {
	valid := `{"version":2,"action":"approve","reason":"Reviewed actual PO terms"}`
	if _, e := AdvancesDecode(strings.NewReader(valid)); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(valid, `"action":"approve"`, `"action":"approve","decision_id":"forged"`, 1), strings.Replace(valid, `"action":"approve"`, `"action":"approve","details":{"kind":"payment"}`, 1), strings.Replace(valid, `"version":2`, `"version":2,"version":3`, 1), valid + valid, `{"version":0,"action":"prepare","reason":"r","details":{"kind":"payment","amount":"999"}}`, `{"version":3,"action":"disburse","reason":"r","payment":{"voucher_ref":"UTR","disbursed_date":"2026-10-11","amount":"999"}}`, `{"version":0,"action":"prepare","reason":"r","details":null}`, strings.Repeat(" ", 65537)} {
		if _, e := AdvancesDecode(strings.NewReader(bad)); e == nil {
			t.Fatal("undeclared authority or amount accepted")
		}
	}
}
