package domain

import "testing"

func TestVendorReviewAssignmentInput(t *testing.T) {
	valid := VendorReviewAssignmentInput{UserID: "11111111-1111-4111-8111-111111111111", Active: true, Reason: "Assign Finance responsibility"}
	if err := VendorReviewAssignmentValidate(valid); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"user", "version", "reason"} {
		in := valid
		switch mode {
		case "user":
			in.UserID = ""
		case "version":
			in.Version = -1
		case "reason":
			in.Reason = " "
		}
		if VendorReviewAssignmentValidate(in) == nil {
			t.Fatal("invalid assignment accepted", mode)
		}
	}
}
