import 'server-only';
import { randomUUID } from 'node:crypto';
import { documentBucket, open, putObject, readStoredObject, seal } from '../../master/documents/files';
import { inspectTechnical } from './evidence-inspector';
import { invalid } from './validation';

export type StageUpload = { name: string; mime: string; data: Buffer };
export type StageEvidence = { id: string; name: string; mime: string; sha256: string; bytes: number };

function key(project: string, pkg: string, kind: string, id: string) {
    return `purchase-evidence/${project}/${pkg}/${kind}/${id}`;
}

export async function saveStageEvidence(project: string, pkg: string, kind: string, file: StageUpload): Promise<StageEvidence> {
    await inspectTechnical(file.name, file.mime, file.data);
    const evidence = seal(file.data), id = randomUUID();
    await putObject(documentBucket(), key(project, pkg, kind, id), evidence.encrypted);
    return { id, name: file.name, mime: file.mime, sha256: evidence.sha256, bytes: file.data.length };
}

export async function readStageEvidence(project: string, pkg: string, kind: string, file: StageEvidence) {
    const data = open(await readStoredObject(documentBucket(), key(project, pkg, kind, file.id)), file.sha256);
    if (data.length !== file.bytes) invalid('file', 'Evidence file failed integrity verification');
    return { file, data };
}
