import 'server-only';
import history from '../../../migrations/history.json';
const latest=Math.max(...Object.keys(history).map(n=>Number(n.split('_')[0])));
import { pool } from '../db';
export async function health(ready: boolean) {
    try {
        if (ready) {
            const row = (await pool().query("SELECT version,dirty FROM schema_migrations")).rows[0];
            if (!row || row.dirty || Number(row.version) !== latest)
                throw Error('Schema not ready');
        }
        return Response.json({ status: 'ok', runtime: 'nextjs' }, { headers: { 'Cache-Control': 'no-store' } });
    }
    catch {
        return Response.json({ status: 'unavailable' }, { status: 503, headers: { 'Cache-Control': 'no-store' } });
    }
}
