import 'server-only';
import { transaction, type DB } from '../db';
import { requireAnyPermission } from '../auth/access';
import { PurchaseError, invalid } from './core/validation';

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
