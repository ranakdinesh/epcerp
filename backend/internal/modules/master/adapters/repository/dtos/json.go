package dtos

import (
	"bytes"
	"encoding/json"

	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func dtoSlice[T, D any](values []T, convert func(T) D) []D {
	if values == nil {
		return nil
	}
	out := make([]D, len(values))
	for i, v := range values {
		out[i] = convert(v)
	}
	return out
}
func dtoPointer[T, D any](v *T, convert func(T) D) *D {
	if v == nil {
		return nil
	}
	out := convert(*v)
	return &out
}

type domainDTO[T any] interface{ Domain() T }

func decodeDomain[T any, D domainDTO[T]](d *json.Decoder, out *T) error {
	var dto D
	if e := d.Decode(&dto); e != nil {
		return e
	}
	*out = dto.Domain()
	return nil
}
func decodeDomainSlice[T any, D domainDTO[T]](d *json.Decoder, out *[]T) error {
	var dto []D
	if e := d.Decode(&dto); e != nil {
		return e
	}
	*out = dtoSlice(dto, func(v D) T { return v.Domain() })
	return nil
}

func TransportValue(value any) any {
	switch v := value.(type) {
	case domain.Category:
		return CategoryDTOFromDomain(v)
	case *domain.Category:
		return dtoPointer(v, CategoryDTOFromDomain)
	case []domain.Category:
		return dtoSlice(v, CategoryDTOFromDomain)

	case domain.CategoryConfiguration:
		return CategoryConfigurationDTOFromDomain(v)
	case *domain.CategoryConfiguration:
		return dtoPointer(v, CategoryConfigurationDTOFromDomain)
	case []domain.CategoryConfiguration:
		return dtoSlice(v, CategoryConfigurationDTOFromDomain)

	case domain.CategoryField:
		return CategoryFieldDTOFromDomain(v)
	case *domain.CategoryField:
		return dtoPointer(v, CategoryFieldDTOFromDomain)
	case []domain.CategoryField:
		return dtoSlice(v, CategoryFieldDTOFromDomain)
	case domain.CategoryOption:
		return CategoryOptionDTOFromDomain(v)
	case *domain.CategoryOption:
		return dtoPointer(v, CategoryOptionDTOFromDomain)
	case []domain.CategoryOption:
		return dtoSlice(v, CategoryOptionDTOFromDomain)

	case domain.CategorySource:
		return CategorySourceDTOFromDomain(v)
	case *domain.CategorySource:
		return dtoPointer(v, CategorySourceDTOFromDomain)
	case []domain.CategorySource:
		return dtoSlice(v, CategorySourceDTOFromDomain)
	case domain.CategorySourceData:
		return CategorySourceDataDTOFromDomain(v)
	case *domain.CategorySourceData:
		return dtoPointer(v, CategorySourceDataDTOFromDomain)
	case []domain.CategorySourceData:
		return dtoSlice(v, CategorySourceDataDTOFromDomain)

	case domain.Item:
		return ItemDTOFromDomain(v)
	case *domain.Item:
		return dtoPointer(v, ItemDTOFromDomain)
	case []domain.Item:
		return dtoSlice(v, ItemDTOFromDomain)

	case domain.ItemDefinition:
		return ItemDefinitionDTOFromDomain(v)
	case *domain.ItemDefinition:
		return dtoPointer(v, ItemDefinitionDTOFromDomain)
	case []domain.ItemDefinition:
		return dtoSlice(v, ItemDefinitionDTOFromDomain)

	case domain.ItemField:
		return ItemFieldDTOFromDomain(v)
	case *domain.ItemField:
		return dtoPointer(v, ItemFieldDTOFromDomain)
	case []domain.ItemField:
		return dtoSlice(v, ItemFieldDTOFromDomain)

	case domain.ItemMakeData:
		return ItemMakeDataDTOFromDomain(v)
	case *domain.ItemMakeData:
		return dtoPointer(v, ItemMakeDataDTOFromDomain)
	case []domain.ItemMakeData:
		return dtoSlice(v, ItemMakeDataDTOFromDomain)
	case domain.ItemOption:
		return ItemOptionDTOFromDomain(v)
	case *domain.ItemOption:
		return dtoPointer(v, ItemOptionDTOFromDomain)
	case []domain.ItemOption:
		return dtoSlice(v, ItemOptionDTOFromDomain)

	case domain.ItemSnapshot:
		return ItemSnapshotDTOFromDomain(v)
	case *domain.ItemSnapshot:
		return dtoPointer(v, ItemSnapshotDTOFromDomain)
	case []domain.ItemSnapshot:
		return dtoSlice(v, ItemSnapshotDTOFromDomain)
	case domain.ManufacturerChoice:
		return ManufacturerChoiceDTOFromDomain(v)
	case *domain.ManufacturerChoice:
		return dtoPointer(v, ManufacturerChoiceDTOFromDomain)
	case []domain.ManufacturerChoice:
		return dtoSlice(v, ManufacturerChoiceDTOFromDomain)
	case domain.ManufacturerData:
		return ManufacturerDataDTOFromDomain(v)
	case *domain.ManufacturerData:
		return dtoPointer(v, ManufacturerDataDTOFromDomain)
	case []domain.ManufacturerData:
		return dtoSlice(v, ManufacturerDataDTOFromDomain)
	case domain.ManufacturerLegacy:
		return ManufacturerLegacyDTOFromDomain(v)
	case *domain.ManufacturerLegacy:
		return dtoPointer(v, ManufacturerLegacyDTOFromDomain)
	case []domain.ManufacturerLegacy:
		return dtoSlice(v, ManufacturerLegacyDTOFromDomain)
	case domain.ManufacturerLegacySource:
		return ManufacturerLegacySourceDTOFromDomain(v)
	case *domain.ManufacturerLegacySource:
		return dtoPointer(v, ManufacturerLegacySourceDTOFromDomain)
	case []domain.ManufacturerLegacySource:
		return dtoSlice(v, ManufacturerLegacySourceDTOFromDomain)

	case domain.Template:
		return TemplateDTOFromDomain(v)
	case *domain.Template:
		return dtoPointer(v, TemplateDTOFromDomain)
	case []domain.Template:
		return dtoSlice(v, TemplateDTOFromDomain)
	case domain.TemplateDefinition:
		return TemplateDefinitionDTOFromDomain(v)
	case *domain.TemplateDefinition:
		return dtoPointer(v, TemplateDefinitionDTOFromDomain)
	case []domain.TemplateDefinition:
		return dtoSlice(v, TemplateDefinitionDTOFromDomain)
	case domain.TemplateField:
		return TemplateFieldDTOFromDomain(v)
	case *domain.TemplateField:
		return dtoPointer(v, TemplateFieldDTOFromDomain)
	case []domain.TemplateField:
		return dtoSlice(v, TemplateFieldDTOFromDomain)
	case domain.TemplateOption:
		return TemplateOptionDTOFromDomain(v)
	case *domain.TemplateOption:
		return dtoPointer(v, TemplateOptionDTOFromDomain)
	case []domain.TemplateOption:
		return dtoSlice(v, TemplateOptionDTOFromDomain)

	case domain.Vendor:
		return VendorDTOFromDomain(v)
	case *domain.Vendor:
		return dtoPointer(v, VendorDTOFromDomain)
	case []domain.Vendor:
		return dtoSlice(v, VendorDTOFromDomain)

	case domain.VendorSource:
		return VendorSourceDTOFromDomain(v)
	case *domain.VendorSource:
		return dtoPointer(v, VendorSourceDTOFromDomain)
	case []domain.VendorSource:
		return dtoSlice(v, VendorSourceDTOFromDomain)

	case domain.VendorVerificationDecision:
		return VendorVerificationDecisionDTOFromDomain(v)
	case *domain.VendorVerificationDecision:
		return dtoPointer(v, VendorVerificationDecisionDTOFromDomain)
	case []domain.VendorVerificationDecision:
		return dtoSlice(v, VendorVerificationDecisionDTOFromDomain)
	case domain.VendorVerificationReview:
		return VendorVerificationReviewDTOFromDomain(v)
	case *domain.VendorVerificationReview:
		return dtoPointer(v, VendorVerificationReviewDTOFromDomain)
	case []domain.VendorVerificationReview:
		return dtoSlice(v, VendorVerificationReviewDTOFromDomain)

	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = TransportValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = TransportValue(item)
		}
		return out
	default:
		return value
	}
}
func MarshalMasterJSON(v any) ([]byte, error) { return json.Marshal(TransportValue(v)) }
func DecodeMasterJSON(d *json.Decoder, out any) error {
	switch v := out.(type) {
	case *domain.Category:
		return decodeDomain[domain.Category, CategoryDTO](d, v)
	case *[]domain.Category:
		return decodeDomainSlice[domain.Category, CategoryDTO](d, v)

	case *domain.CategoryConfiguration:
		return decodeDomain[domain.CategoryConfiguration, CategoryConfigurationDTO](d, v)
	case *[]domain.CategoryConfiguration:
		return decodeDomainSlice[domain.CategoryConfiguration, CategoryConfigurationDTO](d, v)

	case *domain.CategoryField:
		return decodeDomain[domain.CategoryField, CategoryFieldDTO](d, v)
	case *[]domain.CategoryField:
		return decodeDomainSlice[domain.CategoryField, CategoryFieldDTO](d, v)
	case *domain.CategoryOption:
		return decodeDomain[domain.CategoryOption, CategoryOptionDTO](d, v)
	case *[]domain.CategoryOption:
		return decodeDomainSlice[domain.CategoryOption, CategoryOptionDTO](d, v)

	case *domain.CategorySource:
		return decodeDomain[domain.CategorySource, CategorySourceDTO](d, v)
	case *[]domain.CategorySource:
		return decodeDomainSlice[domain.CategorySource, CategorySourceDTO](d, v)
	case *domain.CategorySourceData:
		return decodeDomain[domain.CategorySourceData, CategorySourceDataDTO](d, v)
	case *[]domain.CategorySourceData:
		return decodeDomainSlice[domain.CategorySourceData, CategorySourceDataDTO](d, v)

	case *domain.Item:
		return decodeDomain[domain.Item, ItemDTO](d, v)
	case *[]domain.Item:
		return decodeDomainSlice[domain.Item, ItemDTO](d, v)

	case *domain.ItemDefinition:
		return decodeDomain[domain.ItemDefinition, ItemDefinitionDTO](d, v)
	case *[]domain.ItemDefinition:
		return decodeDomainSlice[domain.ItemDefinition, ItemDefinitionDTO](d, v)

	case *domain.ItemField:
		return decodeDomain[domain.ItemField, ItemFieldDTO](d, v)
	case *[]domain.ItemField:
		return decodeDomainSlice[domain.ItemField, ItemFieldDTO](d, v)

	case *domain.ItemMakeData:
		return decodeDomain[domain.ItemMakeData, ItemMakeDataDTO](d, v)
	case *[]domain.ItemMakeData:
		return decodeDomainSlice[domain.ItemMakeData, ItemMakeDataDTO](d, v)
	case *domain.ItemOption:
		return decodeDomain[domain.ItemOption, ItemOptionDTO](d, v)
	case *[]domain.ItemOption:
		return decodeDomainSlice[domain.ItemOption, ItemOptionDTO](d, v)

	case *domain.ItemSnapshot:
		return decodeDomain[domain.ItemSnapshot, ItemSnapshotDTO](d, v)
	case *[]domain.ItemSnapshot:
		return decodeDomainSlice[domain.ItemSnapshot, ItemSnapshotDTO](d, v)
	case *domain.ManufacturerChoice:
		return decodeDomain[domain.ManufacturerChoice, ManufacturerChoiceDTO](d, v)
	case *[]domain.ManufacturerChoice:
		return decodeDomainSlice[domain.ManufacturerChoice, ManufacturerChoiceDTO](d, v)
	case *domain.ManufacturerData:
		return decodeDomain[domain.ManufacturerData, ManufacturerDataDTO](d, v)
	case *[]domain.ManufacturerData:
		return decodeDomainSlice[domain.ManufacturerData, ManufacturerDataDTO](d, v)
	case *domain.ManufacturerLegacy:
		return decodeDomain[domain.ManufacturerLegacy, ManufacturerLegacyDTO](d, v)
	case *[]domain.ManufacturerLegacy:
		return decodeDomainSlice[domain.ManufacturerLegacy, ManufacturerLegacyDTO](d, v)
	case *domain.ManufacturerLegacySource:
		return decodeDomain[domain.ManufacturerLegacySource, ManufacturerLegacySourceDTO](d, v)
	case *[]domain.ManufacturerLegacySource:
		return decodeDomainSlice[domain.ManufacturerLegacySource, ManufacturerLegacySourceDTO](d, v)

	case *domain.Template:
		return decodeDomain[domain.Template, TemplateDTO](d, v)
	case *[]domain.Template:
		return decodeDomainSlice[domain.Template, TemplateDTO](d, v)
	case *domain.TemplateDefinition:
		return decodeDomain[domain.TemplateDefinition, TemplateDefinitionDTO](d, v)
	case *[]domain.TemplateDefinition:
		return decodeDomainSlice[domain.TemplateDefinition, TemplateDefinitionDTO](d, v)
	case *domain.TemplateField:
		return decodeDomain[domain.TemplateField, TemplateFieldDTO](d, v)
	case *[]domain.TemplateField:
		return decodeDomainSlice[domain.TemplateField, TemplateFieldDTO](d, v)
	case *domain.TemplateOption:
		return decodeDomain[domain.TemplateOption, TemplateOptionDTO](d, v)
	case *[]domain.TemplateOption:
		return decodeDomainSlice[domain.TemplateOption, TemplateOptionDTO](d, v)

	case *domain.Vendor:
		return decodeDomain[domain.Vendor, VendorDTO](d, v)
	case *[]domain.Vendor:
		return decodeDomainSlice[domain.Vendor, VendorDTO](d, v)

	case *domain.VendorSource:
		return decodeDomain[domain.VendorSource, VendorSourceDTO](d, v)
	case *[]domain.VendorSource:
		return decodeDomainSlice[domain.VendorSource, VendorSourceDTO](d, v)

	case *domain.VendorVerificationDecision:
		return decodeDomain[domain.VendorVerificationDecision, VendorVerificationDecisionDTO](d, v)
	case *[]domain.VendorVerificationDecision:
		return decodeDomainSlice[domain.VendorVerificationDecision, VendorVerificationDecisionDTO](d, v)
	case *domain.VendorVerificationReview:
		return decodeDomain[domain.VendorVerificationReview, VendorVerificationReviewDTO](d, v)
	case *[]domain.VendorVerificationReview:
		return decodeDomainSlice[domain.VendorVerificationReview, VendorVerificationReviewDTO](d, v)

	default:
		return d.Decode(out)
	}
}
func UnmarshalMasterJSON(data []byte, out any) error {
	if !json.Valid(data) {
		var value json.RawMessage
		return json.Unmarshal(data, &value)
	}
	return DecodeMasterJSON(json.NewDecoder(bytes.NewReader(data)), out)
}
