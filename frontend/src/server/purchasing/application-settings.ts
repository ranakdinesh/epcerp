import 'server-only';
import { rows, transaction, type DB } from '../db';
import { requireAnyPermission } from '../auth/access';
import { PurchaseError, invalid, string, version } from './core/validation';
import {
    purchaseIndentReadPermissions, purchaseIndentSettingsWrite, purchaseIndentTermsLimit,
    purchaseIndentTermsLineLimit, type PurchaseIndentSettings,
} from '@/lib/purchase-indent-settings';

export async function itemListMode(db: DB, lock = false): Promise<'single' | 'multiple'> {
    const result = await db.query('SELECT item_list_mode FROM application_purchase_settings WHERE singleton' + (lock ? ' FOR SHARE' : ''));
    return result.rows[0]?.item_list_mode === 'multiple' ? 'multiple' : 'single';
}

export async function purchaseSettings(token: string, raw?: Record<string, unknown>) {
    return transaction(async db => {
        const actor = await requireAnyPermission(db, token, ['system:admin'], 'business');
        if (!actor.superuser)
            throw new PurchaseError(403, 'Application settings require a superadmin');
        if (raw) {
            const mode = raw.item_list_mode;
            if (mode !== 'single' && mode !== 'multiple')
                invalid('item_list_mode', 'Choose single or multiple purchase item lists');
            await db.query(
                'UPDATE application_purchase_settings SET item_list_mode=$1,updated_at=clock_timestamp(),updated_by_id=$2 WHERE singleton AND item_list_mode<>$1',
                [mode, actor.id],
            );
        }
        return { item_list_mode: await itemListMode(db) };
    });
}

type IndentDefault = Pick<PurchaseIndentSettings, 'terms' | 'version'>;
const indentColumns = 'indent_default_terms AS terms, indent_terms_version AS version';

export async function indentDefaults(db: DB): Promise<IndentDefault> {
    const [saved] = await rows<IndentDefault>(db, `SELECT ${indentColumns} FROM application_purchase_settings WHERE singleton`);
    if (!saved) throw new Error('Purchase settings singleton is missing');
    return saved;
}

export function purchaseIndentSettings(token: string, raw?: Record<string, unknown>): Promise<PurchaseIndentSettings> {
    return transaction(async db => {
        const user = await requireAnyPermission(db, token,
            raw ? [purchaseIndentSettingsWrite] : [...purchaseIndentReadPermissions, purchaseIndentSettingsWrite],
            raw ? 'business' : false);
        const can_write = user.allows(purchaseIndentSettingsWrite);
        if (!can_write && !purchaseIndentReadPermissions.every(user.allows))
            throw new PurchaseError(403, 'Purchase Indent access is required');
        if (!raw) return { ...await indentDefaults(db), can_write };

        const terms = string(raw.terms, 'terms').replace(/\r\n?/g, '\n').trim();
        if (terms.includes('\0') || terms.length > purchaseIndentTermsLimit || terms.split('\n').length > purchaseIndentTermsLineLimit)
            invalid('terms', 'Use up to 12,000 characters and 1,000 lines of plain text.');
        const expected = version(raw.version);
        const [saved] = await rows<IndentDefault>(db, `
            UPDATE application_purchase_settings
            SET indent_default_terms=$1, indent_terms_version=indent_terms_version+1,
                updated_at=clock_timestamp(), updated_by_id=$2
            WHERE singleton AND indent_terms_version=$3
            RETURNING ${indentColumns}
        `, [terms, user.id, expected]);
        if (!saved) throw new PurchaseError(409, 'The default terms changed. Reload settings before saving.');
        return { ...saved, can_write };
    });
}
