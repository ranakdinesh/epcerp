export type CategoryOption = { code: string; label: string };
export type CategoryField = { code: string; label: string; required: boolean; options: CategoryOption[] };
export type CatalogueRecord = { unit_code?: string; unit_name?: string; child_count?: number;
 id: string; version: number; code: string; name: string; is_active: boolean; category_id?: string; unit_id?: string; configuration?: { fields: CategoryField[]; family_code: string; revision: number; model_required: boolean } };
export type Catalogue = { categories: CatalogueRecord[]; subcategories: CatalogueRecord[]; units: CatalogueRecord[] };
export type CatalogueKind = "categories" | "subcategories" | "units";
