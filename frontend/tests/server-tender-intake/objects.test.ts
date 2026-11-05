import assert from 'node:assert/strict';
import test from 'node:test';
import { createHash, randomUUID } from 'node:crypto';
import { mkdtemp, readdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { writeObject, readObject } from '../../src/server/tenders/intake/objects';

test('incomplete uploads report byte counts, distinguish changed contents, and can be retried', async () => {
  const root = await mkdtemp(join(tmpdir(), 'upload-integrity-'));
  const previousRoot = process.env.EPCERP_DOCUMENT_PACKAGE_ROOT;
  const previousKey = process.env.EPCERP_DOCUMENT_READING_KEY;
  process.env.EPCERP_DOCUMENT_PACKAGE_ROOT = root;
  process.env.EPCERP_DOCUMENT_READING_KEY = 'aa'.repeat(32);
  try {
    const bytes = Buffer.alloc(18 * 1024 * 1024 + 123, 0xa5);
    const hash = createHash('sha256').update(bytes).digest('hex'), key = randomUUID();
    const request = (body: Buffer) => new Request('http://localhost', { method: 'PUT', body: new Uint8Array(body) });
    await assert.rejects(writeObject(key, request(bytes.subarray(0, 10 * 1024 * 1024)), bytes.length, hash), /Upload incomplete: received 10485760 of 18874491 bytes.*Retry/);
    assert.deepEqual(await readdir(root), []);
    await assert.rejects(writeObject(key, request(bytes), bytes.length, '0'.repeat(64)), /received file contents differ/);
    assert.deepEqual(await readdir(root), []);
    await assert.rejects(writeObject(key, request(bytes), bytes.length - 1, hash), /exceeded the selected file size/);
    assert.deepEqual(await readdir(root), []);
    await writeObject(key, request(bytes), bytes.length, hash);
    assert.deepEqual(await readObject(key, bytes.length, hash), bytes);
  } finally {
    if (previousRoot === undefined) delete process.env.EPCERP_DOCUMENT_PACKAGE_ROOT;
    else process.env.EPCERP_DOCUMENT_PACKAGE_ROOT = previousRoot;
    if (previousKey === undefined) delete process.env.EPCERP_DOCUMENT_READING_KEY;
    else process.env.EPCERP_DOCUMENT_READING_KEY = previousKey;
    await rm(root, { recursive: true, force: true });
  }
});
