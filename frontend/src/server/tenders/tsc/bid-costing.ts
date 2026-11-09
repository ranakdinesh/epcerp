import 'server-only';
import { randomUUID } from 'node:crypto';
import { pool, transaction } from '../../db';
import { AppError } from '../../errors';
import { initializeProjectPurchase } from '../../purchasing/project-registration';
import { authorizeTender } from '../documents';

export type BidItem = {
  id: string;
  draft_id: string;
  item_code: string;
  description: string;
  category: string;
  unit: string;
  quantity: number;
  approved_makes: string;
  estimated_rate: number;
  l1_vendor_name: string;
  l1_unit_rate: number;
  l1_total_amount: number;
  sort_order: number;
  created_at: string;
  updated_at: string;
  version: number;
};

export type VendorQuotation = {
  id: string;
  draft_id: string;
  bid_item_id: string;
  vendor_id: string | null;
  vendor_name: string;
  make_offered: string;
  unit_rate: number;
  freight_and_insurance: number;
  gst_percent: number;
  delivery_weeks: number;
  is_technically_compliant: boolean;
  is_l1: boolean;
  remarks: string;
  quote_ref: string;
  created_at: string;
  updated_at: string;
  version: number;
};

export type BidCosting = {
  id: string;
  draft_id: string;
  material_cost: number;
  freight_cost: number;
  civil_and_erection_cost: number;
  contingency_percent: number;
  contingency_amount: number;
  overhead_percent: number;
  overhead_amount: number;
  margin_percent: number;
  margin_amount: number;
  total_bid_value: number;
  status: 'draft' | 'approved' | 'submitted' | 'awarded' | 'lost';
  submission_date: string | null;
  portal_reference: string;
  award_date: string | null;
  contract_reference: string;
  project_id: string | null;
  approved_by_id: string | null;
  approved_at: string | null;
  created_at: string;
  updated_at: string;
  version: number;
};

export async function listBidItems(draftId: string): Promise<BidItem[]> {
  const db = pool();
  const res = await db.query<BidItem>(
    `SELECT id, draft_id, item_code, description, category, unit,
            quantity::float, approved_makes, estimated_rate::float,
            l1_vendor_name, l1_unit_rate::float, l1_total_amount::float,
            sort_order, created_at, updated_at, version
     FROM tender.bid_items
     WHERE draft_id = $1
     ORDER BY sort_order ASC, created_at ASC`,
    [draftId]
  );
  return res.rows;
}

export async function createBidItem(
  draftId: string,
  input: {
    item_code?: string;
    description: string;
    category?: string;
    unit?: string;
    quantity: number;
    approved_makes?: string;
    estimated_rate?: number;
  }
): Promise<BidItem> {
  if (!input.description?.trim()) throw new AppError(422, 'Item description is required');
  const db = pool();
  const res = await db.query<BidItem>(
    `INSERT INTO tender.bid_items(
       draft_id, item_code, description, category, unit, quantity,
       approved_makes, estimated_rate
     ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
     RETURNING id, draft_id, item_code, description, category, unit,
               quantity::float, approved_makes, estimated_rate::float,
               l1_vendor_name, l1_unit_rate::float, l1_total_amount::float,
               sort_order, created_at, updated_at, version`,
    [
      draftId,
      input.item_code?.trim() || '',
      input.description.trim(),
      input.category?.trim() || 'General',
      input.unit?.trim() || 'NOS',
      input.quantity || 1,
      input.approved_makes?.trim() || '',
      input.estimated_rate || 0,
    ]
  );
  return res.rows[0];
}

export async function updateBidItem(
  draftId: string,
  itemId: string,
  input: {
    item_code?: string;
    description?: string;
    category?: string;
    unit?: string;
    quantity?: number;
    approved_makes?: string;
    estimated_rate?: number;
  }
): Promise<BidItem> {
  const db = pool();
  const existingRes = await db.query<BidItem>(
    `SELECT * FROM tender.bid_items WHERE id = $1 AND draft_id = $2`,
    [itemId, draftId]
  );
  const existing = existingRes.rows[0];
  if (!existing) throw new AppError(404, 'Bid item not found');

  const itemCode = input.item_code !== undefined ? input.item_code.trim() : existing.item_code;
  const description = input.description !== undefined ? input.description.trim() : existing.description;
  const category = input.category !== undefined ? input.category.trim() : existing.category;
  const unit = input.unit !== undefined ? input.unit.trim() : existing.unit;
  const quantity = input.quantity !== undefined ? input.quantity : existing.quantity;
  const approvedMakes = input.approved_makes !== undefined ? input.approved_makes.trim() : existing.approved_makes;
  const estimatedRate = input.estimated_rate !== undefined ? input.estimated_rate : existing.estimated_rate;
  const l1Total = Number(existing.l1_unit_rate) * Number(quantity);

  const res = await db.query<BidItem>(
    `UPDATE tender.bid_items
     SET item_code = $1, description = $2, category = $3, unit = $4,
         quantity = $5, approved_makes = $6, estimated_rate = $7,
         l1_total_amount = $8, updated_at = clock_timestamp(), version = version + 1
     WHERE id = $9 AND draft_id = $10
     RETURNING id, draft_id, item_code, description, category, unit,
               quantity::float, approved_makes, estimated_rate::float,
               l1_vendor_name, l1_unit_rate::float, l1_total_amount::float,
               sort_order, created_at, updated_at, version`,
    [
      itemCode,
      description,
      category,
      unit,
      quantity,
      approvedMakes,
      estimatedRate,
      l1Total,
      itemId,
      draftId,
    ]
  );
  return res.rows[0];
}

export async function deleteBidItem(draftId: string, itemId: string): Promise<{ ok: boolean }> {
  const db = pool();
  const res = await db.query(`DELETE FROM tender.bid_items WHERE id = $1 AND draft_id = $2`, [itemId, draftId]);
  if (res.rowCount === 0) throw new AppError(404, 'Bid item not found');
  return { ok: true };
}

export async function listVendorQuotes(draftId: string, itemId?: string): Promise<VendorQuotation[]> {
  const db = pool();
  const params: unknown[] = [draftId];
  let filter = 'draft_id = $1';
  if (itemId) {
    params.push(itemId);
    filter += ' AND bid_item_id = $2';
  }
  const res = await db.query<VendorQuotation>(
    `SELECT id, draft_id, bid_item_id, vendor_id, vendor_name, make_offered,
            unit_rate::float, freight_and_insurance::float, gst_percent::float,
            delivery_weeks, is_technically_compliant, is_l1, remarks, quote_ref,
            created_at, updated_at, version
     FROM tender.vendor_quotations
     WHERE ${filter}
     ORDER BY unit_rate ASC, created_at ASC`,
    params
  );
  return res.rows;
}

export async function saveVendorQuote(
  draftId: string,
  input: {
    id?: string;
    bid_item_id: string;
    vendor_id?: string | null;
    vendor_name: string;
    make_offered?: string;
    unit_rate: number;
    freight_and_insurance?: number;
    gst_percent?: number;
    delivery_weeks?: number;
    is_technically_compliant?: boolean;
    remarks?: string;
    quote_ref?: string;
  }
): Promise<VendorQuotation> {
  if (!input.vendor_name?.trim()) throw new AppError(422, 'Vendor name is required');
  const db = pool();

  if (input.id) {
    const res = await db.query<VendorQuotation>(
      `UPDATE tender.vendor_quotations
       SET vendor_id = $1, vendor_name = $2, make_offered = $3, unit_rate = $4,
           freight_and_insurance = $5, gst_percent = $6, delivery_weeks = $7,
           is_technically_compliant = $8, remarks = $9, quote_ref = $10,
           updated_at = clock_timestamp(), version = version + 1
       WHERE id = $11 AND draft_id = $12
       RETURNING id, draft_id, bid_item_id, vendor_id, vendor_name, make_offered,
                 unit_rate::float, freight_and_insurance::float, gst_percent::float,
                 delivery_weeks, is_technically_compliant, is_l1, remarks, quote_ref,
                 created_at, updated_at, version`,
      [
        input.vendor_id || null,
        input.vendor_name.trim(),
        input.make_offered?.trim() || '',
        input.unit_rate || 0,
        input.freight_and_insurance || 0,
        input.gst_percent || 18,
        input.delivery_weeks || 4,
        input.is_technically_compliant ?? true,
        input.remarks?.trim() || '',
        input.quote_ref?.trim() || '',
        input.id,
        draftId,
      ]
    );
    return res.rows[0];
  }

  const res = await db.query<VendorQuotation>(
    `INSERT INTO tender.vendor_quotations(
       draft_id, bid_item_id, vendor_id, vendor_name, make_offered, unit_rate,
       freight_and_insurance, gst_percent, delivery_weeks, is_technically_compliant,
       remarks, quote_ref
     ) SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
       FROM tender.bid_items WHERE draft_id = $1 AND id = $2
     RETURNING id, draft_id, bid_item_id, vendor_id, vendor_name, make_offered,
               unit_rate::float, freight_and_insurance::float, gst_percent::float,
               delivery_weeks, is_technically_compliant, is_l1, remarks, quote_ref,
               created_at, updated_at, version`,
    [
      draftId,
      input.bid_item_id,
      input.vendor_id || null,
      input.vendor_name.trim(),
      input.make_offered?.trim() || '',
      input.unit_rate || 0,
      input.freight_and_insurance || 0,
      input.gst_percent || 18,
      input.delivery_weeks || 4,
      input.is_technically_compliant ?? true,
      input.remarks?.trim() || '',
      input.quote_ref?.trim() || '',
    ]
  );
  if (!res.rows[0]) throw new AppError(404, 'Bid item not found in this tender');
  return res.rows[0];
}

export async function deleteVendorQuote(draftId: string, quoteId: string): Promise<{ ok: boolean }> {
  const db = pool();
  const res = await db.query(`DELETE FROM tender.vendor_quotations WHERE id = $1 AND draft_id = $2`, [quoteId, draftId]);
  if (res.rowCount === 0) throw new AppError(404, 'Vendor quotation not found');
  return { ok: true };
}

export async function selectL1VendorQuote(
  draftId: string,
  itemId: string,
  quoteId: string
): Promise<{ item: BidItem; quote: VendorQuotation }> {
  return await transaction(async (client) => {
    // Reset is_l1 for all quotes of this item
    await client.query(
      `UPDATE tender.vendor_quotations
       SET is_l1 = false
       WHERE draft_id = $1 AND bid_item_id = $2`,
      [draftId, itemId]
    );

    // Set selected quote as is_l1
    const quoteRes = await client.query<VendorQuotation>(
      `UPDATE tender.vendor_quotations
       SET is_l1 = true, updated_at = clock_timestamp(), version = version + 1
       WHERE id = $1 AND draft_id = $2 AND bid_item_id = $3
       RETURNING id, draft_id, bid_item_id, vendor_id, vendor_name, make_offered,
                 unit_rate::float, freight_and_insurance::float, gst_percent::float,
                 delivery_weeks, is_technically_compliant, is_l1, remarks, quote_ref,
                 created_at, updated_at, version`,
      [quoteId, draftId, itemId]
    );
    const quote = quoteRes.rows[0];
    if (!quote) throw new AppError(404, 'Quotation not found');

    // Update Bid Item L-1 data
    const itemRes = await client.query<BidItem>(
      `UPDATE tender.bid_items
       SET l1_vendor_name = $1, l1_unit_rate = $2,
           l1_total_amount = $2 * quantity,
           updated_at = clock_timestamp(), version = version + 1
       WHERE id = $3 AND draft_id = $4
       RETURNING id, draft_id, item_code, description, category, unit,
                 quantity::float, approved_makes, estimated_rate::float,
                 l1_vendor_name, l1_unit_rate::float, l1_total_amount::float,
                 sort_order, created_at, updated_at, version`,
      [quote.vendor_name, quote.unit_rate, itemId, draftId]
    );
    const item = itemRes.rows[0];

    return { item, quote };
  });
}

const costingColumns = `id, draft_id, material_cost::float, freight_cost::float,
            civil_and_erection_cost::float, contingency_percent::float,
            contingency_amount::float, overhead_percent::float, overhead_amount::float,
            margin_percent::float, margin_amount::float, total_bid_value::float,
            status, submission_date, portal_reference, award_date, contract_reference,
            project_id, approved_by_id, approved_at, created_at, updated_at, version`;

export async function getBidCosting(draftId: string): Promise<BidCosting | null> {
  const db = pool();
  // Compute sum of L-1 amounts from bid_items
  const sumRes = await db.query<{ total: number }>(
    `SELECT COALESCE(SUM(l1_total_amount), 0)::float as total
     FROM tender.bid_items
     WHERE draft_id = $1`,
    [draftId]
  );
  const calculatedMaterialCost = sumRes.rows[0]?.total ?? 0;

  const res = await db.query<BidCosting>(
    `SELECT ${costingColumns}
     FROM tender.bid_costings
     WHERE draft_id = $1`,
    [draftId]
  );

  if (res.rows[0]) {
    const current = res.rows[0];
    if (current.status === 'draft' && current.material_cost !== calculatedMaterialCost) {
      current.material_cost = calculatedMaterialCost;
    }
    return current;
  }

  return {
    id: '',
    draft_id: draftId,
    material_cost: calculatedMaterialCost,
    freight_cost: 0,
    civil_and_erection_cost: 0,
    contingency_percent: 3,
    contingency_amount: (calculatedMaterialCost * 0.03),
    overhead_percent: 5,
    overhead_amount: (calculatedMaterialCost * 0.05),
    margin_percent: 10,
    margin_amount: (calculatedMaterialCost * 0.1),
    total_bid_value: calculatedMaterialCost * 1.18,
    status: 'draft',
    submission_date: null,
    portal_reference: '',
    award_date: null,
    contract_reference: '',
    project_id: null,
    approved_by_id: null,
    approved_at: null,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    version: 0,
  };
}

export async function saveBidCosting(
  draftId: string,
  input: {
    material_cost?: number;
    freight_cost?: number;
    civil_and_erection_cost?: number;
    contingency_percent?: number;
    overhead_percent?: number;
    margin_percent?: number;
    status?: BidCosting['status'];
    submission_date?: string | null;
    portal_reference?: string;
  },
  actorId: string
): Promise<BidCosting> {
  const db = pool();
  const materialCost = input.material_cost ?? 0;
  const freightCost = input.freight_cost ?? 0;
  const civilCost = input.civil_and_erection_cost ?? 0;
  const contingencyPct = input.contingency_percent ?? 3;
  const overheadPct = input.overhead_percent ?? 5;
  const marginPct = input.margin_percent ?? 10;

  const baseDirect = materialCost + freightCost + civilCost;
  const contingencyAmt = baseDirect * (contingencyPct / 100);
  const overheadAmt = baseDirect * (overheadPct / 100);
  const subtotal = baseDirect + contingencyAmt + overheadAmt;
  const marginAmt = subtotal * (marginPct / 100);
  const totalBid = subtotal + marginAmt;

  const status = input.status || 'draft';
  if (status === 'awarded') throw new AppError(422, 'Use the award handoff to create a project');
  const submissionDate = input.submission_date || null;
  const portalRef = input.portal_reference?.trim() || '';

  const res = await db.query<BidCosting>(
    `INSERT INTO tender.bid_costings(
       draft_id, material_cost, freight_cost, civil_and_erection_cost,
       contingency_percent, contingency_amount, overhead_percent, overhead_amount,
       margin_percent, margin_amount, total_bid_value, status,
       submission_date, portal_reference, approved_by_id, approved_at
     ) VALUES (
       $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
       CASE WHEN $12::varchar = 'approved' THEN $15::uuid ELSE NULL END,
       CASE WHEN $12::varchar = 'approved' THEN clock_timestamp() ELSE NULL END
     )
     ON CONFLICT (draft_id) DO UPDATE
     SET material_cost = EXCLUDED.material_cost,
         freight_cost = EXCLUDED.freight_cost,
         civil_and_erection_cost = EXCLUDED.civil_and_erection_cost,
         contingency_percent = EXCLUDED.contingency_percent,
         contingency_amount = EXCLUDED.contingency_amount,
         overhead_percent = EXCLUDED.overhead_percent,
         overhead_amount = EXCLUDED.overhead_amount,
         margin_percent = EXCLUDED.margin_percent,
         margin_amount = EXCLUDED.margin_amount,
         total_bid_value = EXCLUDED.total_bid_value,
         status = EXCLUDED.status,
         submission_date = EXCLUDED.submission_date,
         portal_reference = EXCLUDED.portal_reference,
         approved_by_id = CASE WHEN EXCLUDED.status = 'approved' THEN $15::uuid ELSE tender.bid_costings.approved_by_id END,
         approved_at = CASE WHEN EXCLUDED.status = 'approved' THEN clock_timestamp() ELSE tender.bid_costings.approved_at END,
         updated_at = clock_timestamp(),
         version = tender.bid_costings.version + 1
     WHERE tender.bid_costings.project_id IS NULL
     RETURNING id, draft_id, material_cost::float, freight_cost::float,
               civil_and_erection_cost::float, contingency_percent::float,
               contingency_amount::float, overhead_percent::float, overhead_amount::float,
               margin_percent::float, margin_amount::float, total_bid_value::float,
               status, submission_date, portal_reference, award_date, contract_reference,
               project_id, approved_by_id, approved_at, created_at, updated_at, version`,
    [
      draftId,
      materialCost,
      freightCost,
      civilCost,
      contingencyPct,
      contingencyAmt,
      overheadPct,
      overheadAmt,
      marginPct,
      marginAmt,
      totalBid,
      status,
      submissionDate,
      portalRef,
      actorId,
    ]
  );
  if (!res.rows[0]) throw new AppError(409, 'Awarded costing cannot be edited');
  return res.rows[0];
}

export async function awardAndCreateProject(
  token: string,
  draftId: string,
  input: { project_title: string; contract_reference: string; client_name: string }
): Promise<{ project_id: string; project_number: string; costing: BidCosting }> {
  const title = typeof input.project_title === 'string' ? input.project_title.trim() : '';
  const reference = typeof input.contract_reference === 'string' ? input.contract_reference.trim() : '';
  const clientName = typeof input.client_name === 'string' ? input.client_name.trim() : '';
  if (title.length < 3 || title.length > 248 || !reference || reference.length > 255 || !clientName || clientName.length > 255) {
    throw new AppError(422, 'Enter a project title, contract reference and registered client name');
  }
  return transaction(async (client) => {
    const { actor } = await authorizeTender(client, token, draftId, 'write');
    if (!['projects.master.read', 'projects.master.write', 'projects.portfolio.write', 'projects.purchase_assignments.write'].every(permission => actor.allows(permission))) {
      throw new AppError(403, 'Project creation and purchase assignment permissions are required');
    }
    const managerId = actor.id;
    const costing = (await client.query<BidCosting>(
      `SELECT ${costingColumns} FROM tender.bid_costings WHERE draft_id = $1 FOR UPDATE`,
      [draftId]
    )).rows[0];
    if (!costing) throw new AppError(422, 'Bid costing must be completed before project award handoff');
    if (costing.project_id) {
      if (costing.contract_reference !== reference) throw new AppError(409, 'This tender already has an award project');
      const project = (await client.query<{ project_number: string }>(
        'SELECT project_number FROM public.projects WHERE id = $1', [costing.project_id]
      )).rows[0];
      return { project_id: costing.project_id, project_number: project.project_number, costing };
    }
    if (costing.status !== 'submitted') throw new AppError(422, 'Record portal submission before awarding this tender');
    const matches = await client.query<{ id: string }>(
      `SELECT id FROM public.clients WHERE is_active AND
       (lower(display_name) = lower($1) OR lower(legal_name) = lower($1)) LIMIT 2 FOR SHARE`,
      [clientName]
    );
    if (matches.rows.length !== 1) throw new AppError(422, 'Enter the exact name of one active registered client');
    const clientId = matches.rows[0].id;

    // 4. Ensure unique normalized project name
    const baseName = title;
    let name = baseName;
    let normalizedName = baseName.toLowerCase();
    const existingProj = await client.query(
      `SELECT id FROM public.projects WHERE normalized_name = $1`,
      [normalizedName]
    );
    if (existingProj.rowCount && existingProj.rowCount > 0) {
      const suffix = Date.now().toString(36).slice(-4);
      name = `${baseName} (${suffix})`;
      normalizedName = `${normalizedName} (${suffix})`.toLowerCase();
    }

    // 5. Create Project in public.projects
    const projectId = randomUUID();
    const projRes = await client.query<{ id: string; project_number: string }>(
      `WITH number AS (SELECT nextval('project_number_sequence') AS value)
       INSERT INTO public.projects(
         id, project_number, name, normalized_name, client_id, project_manager_id,
         lifecycle_status, is_active, version, created_by_id, updated_by_id,
         purchase_enabled, contract_value, po_number, currency
       )
       SELECT
         $1,
         'PRJ-' || CASE WHEN value < 1000000 THEN lpad(value::text, 6, '0') ELSE value::text END,
         $2,
         $3,
         $4,
         $5,
         'active',
         true,
         1,
         $6,
         $6,
         true,
         $7,
         $8,
         'INR'
       FROM number
       RETURNING id, project_number`,
      [
        projectId,
        name,
        normalizedName,
        clientId,
        managerId,
        managerId,
        costing.total_bid_value,
        reference,
      ]
    );
    const createdProject = projRes.rows[0];

    // 6. Project manager history entry
    const pmHistoryId = randomUUID();
    await client.query(
      `INSERT INTO public.project_manager_history(
         id, project_id, previous_manager_id, project_manager_id, project_version,
         reason, created_at, created_by_id
       ) VALUES (
         $1, $2, NULL, $3, 1, 'Initial project manager assignment upon tender award', clock_timestamp(), $4
       )`,
      [pmHistoryId, projectId, managerId, managerId]
    );

    await initializeProjectPurchase(client, projectId, managerId, actor.id);

    // Transfer source BOQ lines for catalogue and required-date review.
    const boListId = randomUUID();
    await client.query(
      `INSERT INTO public.purchase_bo_lists(
         id, project_id, preparer_id, name, state, is_active, version,
         created_by_id, updated_by_id
       ) VALUES (
         $1, $2, $3, $4, 'draft', true, 1, $3, $3
       )`,
      [boListId, projectId, managerId, 'Tender Award BOQ']
    );

    // 9. Transfer all tender bid items to purchase_bo_lines
    const bidItems = await client.query<BidItem>(
      `SELECT id, item_code, description, category, unit, quantity::float, approved_makes,
              estimated_rate::float, l1_vendor_name, l1_unit_rate::float, l1_total_amount::float, sort_order
       FROM tender.bid_items
       WHERE draft_id = $1
       ORDER BY sort_order ASC, created_at ASC`,
      [draftId]
    );

    for (let i = 0; i < bidItems.rows.length; i++) {
      const item = bidItems.rows[i];
      const lineId = randomUUID();
      const itemSnapshot = {
        sku: item.item_code,
        uom: item.unit,
        item_code: item.item_code,
        description: item.description,
        category: item.category,
        unit: item.unit,
        approved_makes: item.approved_makes,
        estimated_rate: item.estimated_rate,
        l1_vendor_name: item.l1_vendor_name,
        l1_unit_rate: item.l1_unit_rate,
        l1_total_amount: item.l1_total_amount,
      };
      await client.query(
        `INSERT INTO public.purchase_bo_lines(
           id, bo_id, quantity, reference, notes, item_snapshot, sort_order, is_active,
           created_by_id, updated_by_id
         ) VALUES (
           $1, $2, $3, $4, $5, $6, $7, true, $8, $8
         )`,
        [
          lineId,
          boListId,
          Math.max(0.001, Number(item.quantity) || 1),
          item.item_code || '',
          item.description || '',
          JSON.stringify(itemSnapshot),
          item.sort_order || i + 1,
          managerId,
        ]
      );
    }

    // 10. Purchase BO audit event
    const boEventId = randomUUID();
    await client.query(
      `INSERT INTO public.purchase_bo_events(
         id, subject_id, action, reason, before_snapshot, after_snapshot, created_by_id
       ) VALUES (
         $1, $2, 'created_from_tender', 'Generated from Tender Award BOQ', '{}'::jsonb, $3::jsonb, $4
       )`,
      [
        boEventId,
        boListId,
        JSON.stringify({ item_count: bidItems.rows.length, total_value: costing.total_bid_value }),
        managerId,
      ]
    );

    // 11. Update tender.bid_costings with status='awarded' and project_id
    const updatedCostingRes = await client.query<BidCosting>(
      `UPDATE tender.bid_costings
       SET status = 'awarded', award_date = clock_timestamp()::date,
           contract_reference = $1, project_id = $2,
           updated_at = clock_timestamp(), version = version + 1
       WHERE draft_id = $3
       RETURNING id, draft_id, material_cost::float, freight_cost::float,
                 civil_and_erection_cost::float, contingency_percent::float,
                 contingency_amount::float, overhead_percent::float, overhead_amount::float,
                 margin_percent::float, margin_amount::float, total_bid_value::float,
                 status, submission_date, portal_reference, award_date, contract_reference,
                 project_id, approved_by_id, approved_at, created_at, updated_at, version`,
      [reference, projectId, draftId]
    );

    return {
      project_id: projectId,
      project_number: createdProject.project_number,
      costing: updatedCostingRes.rows[0],
    };
  });
}
