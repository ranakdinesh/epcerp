import type { DB } from '../../db';
import type { Actor } from './access';
import { project } from './access';
import { scoped } from '../sourcing/context';
import { stepAccess } from '../workflow/reader';
export async function stageAccess(db: DB, a: Actor, p: string, id: string, prefix: string, stage: number) { await project(db, p, true); if (!a.allows(prefix + '.read'))
    return { read: false, write: false, scoped: false, work: false }; const scope = await scoped(db, a, p, prefix + '.read', false), task = await stepAccess(db, a, p, id, stage), read = scope || task.read; return { read, write: read && a.allows(prefix + '.write'), scoped: scope, work: task.work }; }
