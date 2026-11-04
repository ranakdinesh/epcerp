// Command catalogue-compatibility is an offline parity oracle; it never opens a database.
package main

import (
	"bytes"
	"encoding/json"
	httpDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/httpx/dtos"
	repo "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository"
	d "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"golang.org/x/text/cases"
	"os"
	"unicode"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "fold" {
		pairs := [][2]any{}
		fold := cases.Fold()
		for r := rune(0); r <= unicode.MaxRune; r++ {
			v := fold.String(string(r))
			if v != string(r) {
				pairs = append(pairs, [2]any{int(r), v})
			}
		}
		_ = json.NewEncoder(os.Stdout).Encode(pairs)
		return
	}
	var inputs []struct {
		Area          string            `json:"area"`
		Kind          string            `json:"kind"`
		Body          json.RawMessage   `json:"body"`
		Configuration json.RawMessage   `json:"configuration"`
		Manufacturer  json.RawMessage   `json:"manufacturer"`
		Values        map[string]string `json:"values"`
		Model         string            `json:"model"`
	}
	if e := json.NewDecoder(os.Stdin).Decode(&inputs); e != nil {
		panic(e)
	}
	out := []map[string]any{}
	for _, in := range inputs {
		r := map[string]any{}
		var err error
		decode := func(v any) error {
			dec := json.NewDecoder(bytes.NewReader(in.Body))
			dec.DisallowUnknownFields()
			dec.UseNumber()
			return httpDTO.DecodeMasterJSON(dec, v)
		}
		switch in.Area {
		case "vendor-create":
			var v d.VendorCreate
			err = decode(&v)
			if err == nil {
				v.Code = d.VendorNormalizeCode(v.Code)
				v.Name = d.VendorNormalizeName(v.Name)
				err = d.VendorValidateMaster(v.Code, v.Name, v.Data)
			}
			if err == nil {
				err = d.VendorValidateReason(v.Reason)
			}
			if err == nil {
				r["value"] = httpDTO.TransportValue(v)
				r["fingerprint"] = repo.VendorValues{}.Fingerprint(v.Data)
			}
		case "vendor-update":
			var v d.VendorUpdate
			err = decode(&v)
			if err == nil {
				err = d.VendorValidateReason(v.Reason)
			}
			if err == nil {
				v.Name = d.VendorNormalizeName(v.Name)
				err = d.VendorValidateMaster(in.Kind, v.Name, v.Data)
			}
			if err == nil {
				r["value"] = httpDTO.TransportValue(v)
				r["fingerprint"] = repo.VendorValues{}.Fingerprint(v.Data)
			}
		case "vendor-import":
			var v d.VendorInput
			err = decode(&v)
			if err == nil {
				err = d.VendorValidate(v)
			}
			if err == nil {
				r["value"] = httpDTO.TransportValue(v)
				r["fingerprint"] = repo.VendorValues{}.Fingerprint(v.Data)
			}
		case "category-create":
			var v d.CategoryCreate
			err = decode(&v)
			if err == nil {
				v, err = d.CategoryCleanCreate(in.Kind, v)
			}
			r["value"] = httpDTO.TransportValue(v)
		case "category-edit":
			var v d.CategoryEdit
			err = decode(&v)
			if err == nil {
				v, err = d.CategoryCleanEdit(in.Kind, v)
			}
			r["value"] = httpDTO.TransportValue(v)
		case "category-source":
			var v d.CategorySource
			err = decode(&v)
			if err == nil {
				r["fingerprint"] = repo.CategoryValues{}.Fingerprint(v)
			}
		case "manufacturer":
			v, _, e := httpDTO.ParseManufacturer(bytes.NewReader(in.Body), in.Kind)
			err = e
			r["value"] = map[string]any{"code": v.Code, "name": v.Name, "data": v.Data.Snapshot(), "reason": v.Reason, "version": v.Version, "is_active": v.Active}
		case "manufacturer-import":
			var v d.ManufacturerLegacy
			err = decode(&v)
			if err == nil {
				err = d.ManufacturerValidateImport(v)
				r["fingerprint"] = repo.ManufacturerSystemValues{}.Fingerprint(v)
			}
		case "template":
			var v d.TemplateDefinition
			err = decode(&v)
			if err == nil {
				err = d.TemplateValidate([]d.TemplateDefinition{v})
			}
			if err == nil {
				r["value"] = httpDTO.TransportValue(v)
				r["fingerprint"] = repo.TemplateValues{}.Fingerprint(v)
			}
		case "preview":
			var v d.Template
			err = decode(&v)
			if err == nil {
				var p d.TemplatePreview
				p, err = d.TemplateEvaluate(v, in.Values)
				if err == nil {
					r["description"] = p.Description
					r["engineering_key"] = repo.TemplateValues{}.EngineeringKey(p.Identity)
				}
			}
		case "item":
			var c d.ItemConfiguration
			var m d.ItemMake
			err = httpDTO.UnmarshalMasterJSON(in.Configuration, &c)
			if err == nil {
				err = httpDTO.UnmarshalMasterJSON(in.Manufacturer, &m)
			}
			if err == nil {
				var derived d.ItemDerived
				derived, err = d.ItemDerive(c, m, in.Values, repo.ItemValues{}.NormalizeModel(in.Model))
				if err == nil {
					eng, key := repo.ItemValues{}.Keys(derived.Identity)
					r["engineering_key"] = eng
					r["fingerprint"] = key
					r["model"] = derived.Model
					r["prefix"] = derived.Prefix
					r["description"] = derived.Description
					r["specifications"] = derived.Specifications
				}
			}
		}
		r["valid"] = err == nil
		out = append(out, r)
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
}
