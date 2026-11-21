import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { evaluate, fingerprint, validateReview, type Snapshot, type Review } from '../../src/server/master/verification/domain';
test('Finance fingerprints, projected document decisions and purchasing eligibility match Go', { skip: !process.env.EPCERP_VERIFICATION_COMPATIBILITY_BINARY }, () => {
    const base: Snapshot = { id: '00112233-4455-4677-8899-aabbccddeeff', version: 1, active: true, gst_registered: false, documents: [{ id: '11112233-4455-4677-8899-aabbccddeeff', version: 1, doc_type: 'pan', sha256: '0'.repeat(64), previewable: true, approval_known: false, approval: '', approval_comment: '' }, { id: '22112233-4455-4677-8899-aabbccddeeff', version: 1, doc_type: 'cheque', sha256: 'a'.repeat(64), previewable: true, approval_known: false, approval: '', approval_comment: '' }] };
    const fixtures: {
        s: Snapshot;
        r: Review | null;
        c: Parameters<typeof validateReview>[1];
    }[] = [];
    for (const active of [true, false])
        for (const gst_registered of [true, false])
            for (const mode of ['none', 'partial', 'accept', 'reject', 'stale', 'pending', 'unreadable', 'missing', 'extra']) {
                const s = structuredClone({ ...base, active, gst_registered });
                if (mode === 'missing')
                    s.documents.pop();
                if (mode === 'extra')
                    s.documents.push({ ...s.documents[0], id: '33112233-4455-4677-8899-aabbccddeeff', doc_type: 'other' });
                if (mode === 'unreadable')
                    s.documents[0].previewable = false;
                if (mode === 'pending')
                    for (const d of s.documents) {
                        d.approval_known = true;
                        d.approval = 'pending';
                    }
                if (['accept', 'reject'].includes(mode))
                    for (const d of s.documents) {
                        d.approval_known = true;
                        d.approval = mode;
                        d.approval_comment = 'Persisted document decision';
                    }
                const decisions = s.documents.slice(0, mode === 'partial' ? 1 : undefined).map(d => ({ document_id: d.id, decision: 'accept', comment: 'Legacy approved' })), r: Review | null = mode === 'none' ? null : { supplier_id: s.id, version: 3, fingerprint: mode === 'stale' ? 'outdated' : fingerprint(s), decisions, reason: 'Legacy review', reviewed_at: '2020-01-01T00:00:00Z', reviewed_by_id: null };
                fixtures.push({ s, r, c: { version: 3, fingerprint: fingerprint(s), decisions, reason: 'QA reason' } });
            }
    const s = structuredClone(base);
    for (const mode of ['empty', 'duplicate', 'foreign', 'blank_reason', 'blank_comment', 'long_comment', 'invalid_decision']) {
        const c = { version: 0, fingerprint: fingerprint(s), decisions: s.documents.map(d => ({ document_id: d.id, decision: 'accept', comment: 'QA' })), reason: 'QA' };
        if (mode === 'empty')
            c.decisions = [];
        if (mode === 'duplicate')
            c.decisions[1] = c.decisions[0];
        if (mode === 'foreign')
            c.decisions[0].document_id = 'foreign';
        if (mode === 'blank_reason')
            c.reason = ' ';
        if (mode === 'blank_comment')
            c.decisions[0].comment = ' ';
        if (mode === 'long_comment')
            c.decisions[0].comment = 'é'.repeat(1001);
        if (mode === 'invalid_decision')
            c.decisions[0].decision = 'approved';
        fixtures.push({ s, r: null, c });
    }
    const inputs = fixtures.map(({ s, r, c }) => ({ Snapshot: { ID: s.id, Version: s.version, Active: s.active, GSTRegistered: s.gst_registered, Documents: s.documents.map(d => ({ ID: d.id, DocType: d.doc_type, SHA256: d.sha256, Version: d.version, Previewable: d.previewable, ApprovalKnown: d.approval_known, Approval: d.approval, ApprovalComment: d.approval_comment })) }, Review: r ? { SupplierID: r.supplier_id, Version: r.version, Fingerprint: r.fingerprint, Decisions: r.decisions?.map(d => ({ DocumentID: d.document_id, Decision: d.decision, Comment: d.comment })), Reason: r.reason, UpdatedAt: r.reviewed_at, UpdatedBy: r.reviewed_by_id } : null, Command: { Version: c.version, Fingerprint: c.fingerprint, Reason: c.reason, Decisions: c.decisions.map(d => ({ DocumentID: d.document_id, Decision: d.decision, Comment: d.comment })) } }));
    const go = JSON.parse(execFileSync(process.env.EPCERP_VERIFICATION_COMPATIBILITY_BINARY!, [], { input: JSON.stringify(inputs) }).toString());
    fixtures.forEach(({ s, r, c }, i) => { assert.equal(fingerprint(s), go[i].fingerprint); assert.deepEqual(evaluate(s, r), go[i].value, 'status fixture ' + i); let error: unknown; try {
        validateReview(s, c);
    }
    catch (e) {
        error = e;
    } assert.equal((error as Error)?.message, go[i].error, 'validation fixture ' + i); });
});

test('supplier profile, document and Finance ownership switches remain independent',async()=>{
 const {ownsNativeSupplierPath}=await import('../../src/server/master/http');
 const names=['EPCERP_VENDOR_PROFILES_OWNER','EPCERP_VENDOR_DOCUMENTS_OWNER','EPCERP_VENDOR_VERIFICATION_OWNER'],before=names.map(n=>process.env[n]),id='00112233-4455-4677-8899-aabbccddeeff';
 try{for(let mask=0;mask<8;mask++){names.forEach((name,i)=>{process.env[name]=mask&(1<<i)?'nextjs':'go';});assert.equal(ownsNativeSupplierPath('/suppliers'),!!(mask&1));assert.equal(ownsNativeSupplierPath(`/suppliers/${id}/documents`),!!(mask&2));for(const path of [`/suppliers/${id}/verification`,`/suppliers/${id}/verification/assignment`,`/suppliers/${id}/verification/assignment/options`,`/suppliers/${id}/documents/reconcile-approvals`])assert.equal(ownsNativeSupplierPath(path),!!(mask&4));}}
 finally{names.forEach((name,i)=>{if(before[i]===undefined)delete process.env[name];else process.env[name]=before[i];});}
});
