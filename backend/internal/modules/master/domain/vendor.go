package domain

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type VendorSource struct {
	ID        string
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
}
type Vendor struct {
	ID        string
	Code      string
	Name      string
	Data      map[string]any
	Active    bool
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy *string
	UpdatedBy *string
	Source    VendorSource
	Imported  bool
}
type VendorInput struct {
	Source VendorSource
	Code   string
	Name   string
	Data   map[string]any
	Active *bool
}

var VendorUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var VendorForbidden = &Error{Kind: AccessDenied, Detail: "Insufficient permission"}
var VendorMissing = &Error{Kind: NotFound, Detail: "Supplier not found"}
var VendorConflict = &Error{Kind: Conflict, Detail: "Source import conflicts with an existing supplier"}

func VendorInvalid(s string) error { return &Error{Kind: InvalidInput, Detail: s} }

var vendorUuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func VendorValidID(s string) bool { return vendorUuid.MatchString(s) }

var vendorCode = regexp.MustCompile(`^[A-Z0-9_-]{1,16}$`)

func VendorValidate(in VendorInput) error {
	if !VendorValidID(in.Source.ID) || in.Source.Version < 1 || in.Source.CreatedAt.IsZero() || in.Source.UpdatedAt.IsZero() || in.Active == nil {
		return VendorInvalid("Source identity, revision, timestamps and status are required")
	}
	return VendorValidateMaster(in.Code, in.Name, in.Data)
}
func VendorValidateMaster(c, name string, data map[string]any) error {
	in := VendorInput{Code: c, Name: name, Data: data}
	if !vendorCode.MatchString(in.Code) {
		return VendorFieldError("code", "Use 1–16 letters, numbers, hyphens or underscores")
	}
	if strings.TrimSpace(in.Name) == "" || utf8.RuneCountInString(in.Name) > 255 {
		return VendorFieldError("name", "Supplier name is required (maximum 255 characters)")
	}
	if in.Data == nil {
		return VendorInvalid("Supplier data is required")
	}
	fields := map[string]int{"registration_reference": 120, "contact": 255, "location": 255, "contact_person_name": 255, "email": 255, "phone": 50, "aadhar_number": 50, "pan_number": 50, "gst_number": 50, "cancelled_cheque_ref": 120, "notes": 2000, "indent_notes": 4000, "indent_terms": 12000, "delivery_terms": 2000}
	for key, value := range in.Data {
		if maximum, ok := fields[key]; ok {
			s, valid := value.(string)
			if !valid || utf8.RuneCountInString(s) > maximum {
				return VendorFieldError(key, "Invalid supplier text field")
			}
			continue
		}
		switch key {
		case "qualification":
			s, ok := value.(string)
			if !ok || (s != "unverified" && s != "approved" && s != "suspended") {
				return VendorInvalid("Invalid qualification")
			}
		case "onboarding_verified":
			if _, ok := value.(bool); !ok {
				return VendorInvalid("Invalid onboarding status")
			}
		case "category_ids", "item_ids":
			if values, ok := value.([]any); !ok || len(values) != 0 {
				return VendorInvalid("Catalogue links require a reviewed Go identifier mapping")
			}
		case "payment_terms":
			if e := VendorValidateTerms(value); e != nil {
				return e
			}
		default:
			return VendorInvalid("Unknown supplier field")
		}
	}
	if in.Data["qualification"] == "approved" {
		notes, _ := in.Data["notes"].(string)
		if strings.TrimSpace(notes) == "" {
			return VendorFieldError("notes", "Approved qualification requires review notes")
		}
	}
	return nil
}
func VendorValidateTerms(value any) error {
	terms, ok := value.(map[string]any)
	if !ok {
		return VendorInvalid("Invalid payment terms")
	}
	total := new(big.Rat)
	for key, v := range terms {
		switch key {
		case "advance_pct", "qc_pct", "delivery_pct", "retention_pct":
			if _, isText := v.(string); isText {
				return VendorInvalid("Payment values must be numbers")
			}
			n, ok := new(big.Rat).SetString(fmt.Sprint(v))
			if !ok || n.Sign() < 0 || n.Cmp(big.NewRat(100, 1)) > 0 {
				return VendorInvalid("Invalid payment percentage")
			}
			total.Add(total, n)
		case "credit_days":
			if _, isText := v.(string); isText {
				return VendorInvalid("Payment values must be numbers")
			}
			n, ok := new(big.Rat).SetString(fmt.Sprint(v))
			if !ok || !n.IsInt() || n.Sign() < 0 || n.Cmp(big.NewRat(365, 1)) > 0 {
				return VendorInvalid("Invalid credit days")
			}
		case "description":
			s, ok := v.(string)
			if !ok || utf8.RuneCountInString(s) > 500 {
				return VendorInvalid("Invalid payment description")
			}
		default:
			return VendorInvalid("Unknown payment term")
		}
	}
	difference := new(big.Rat).Sub(total, big.NewRat(100, 1))
	difference.Abs(difference)
	if total.Sign() > 0 && difference.Cmp(big.NewRat(1, 100)) > 0 {
		return VendorFieldError("advance_pct", "Payment percentages must total 100")
	}
	return nil
}

func VendorFieldError(field, message string) error {
	return &Error{Kind: InvalidInput, Detail: message, Field: field}
}
