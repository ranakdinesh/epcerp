export const definition = () => ({ model_required: true, subcategory_code: "QATYPE", template_name: "QA <Template> Σς", fields: [{ field_code: "SIZE", field_name: "Size", input_type: "dropdown", mandatory: true, allow_na: true, use_in_description: true, description_order: 1, description_prefix: "Size ", description_suffix: " mm", use_in_duplicate_check: true, sort_order: 1, is_active: true, options: [{ code: "S1", label: "10", is_active: true, sort_order: 1 }, { code: "S2", label: "20", is_active: true, sort_order: 2 }] }] });

/** Make-linked catalogue rows model issued records from before manufacturer-free SKU creation. */
export async function legacyCatalogueItem(
    configurationId: string,
    make: import("../../src/server/master/items/domain").ItemMake,
    model: string,
) {
    const { randomUUID } = await import("node:crypto");
    const { transaction } = await import("../../src/server/db");
    const { ItemRepository } = await import("../../src/server/master/items/repository");
    const { deriveItem } = await import("../../src/server/master/items/domain");
    return transaction(async db => {
        const repo = new ItemRepository(db);
        const configuration = await repo.configuration(configurationId);
        const item = deriveItem(configuration, { SIZE: "S1" }, model);
        const id = randomUUID();
        const sku = item.prefix + "-" + make.code;
        const snapshot = {
            configuration: { ...configuration.data, name: configuration.name, code: configuration.code },
            manufacturer: make,
            sku_prefix: sku,
        };
        await db.query(`INSERT INTO catalogue_items
            (id, configuration_id, manufacturer_id, category_id, model, specifications, snapshot, fingerprint,
             engineering_key, description, sku, status, is_active)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'approved',true)`,
        [id, configurationId, make.id, configuration.category_id, item.model, item.specifications,
            snapshot, randomUUID().replaceAll("-", "").padEnd(64, "0"), item.engineering_key,
            item.description + " · " + make.name, sku]);
        return repo.get(id);
    });
}
