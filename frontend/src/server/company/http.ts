import 'server-only';
import { AppError } from '../errors';
import { pool, transaction } from '../db';
import { requireAnyPermission } from '../auth/access';
import { body } from '../auth/validation';
import * as service from './service';

export const ownsCompanyPath = (path: string) => /^\/settings\/company-profile(?:\/|$)/.test(path);

const headers = { 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff' };
const json = (val: unknown, status = 200) => Response.json(val, { status, headers });

export async function handleCompany(request: Request): Promise<Response> {
  try {
    const url = new URL(request.url);
    const path = url.pathname.replace(/^\/api\/v1/, '');
    const parts = path.split('/').filter(Boolean); // ['settings', 'company-profile', ...]
    const token = /^Bearer ([^\s]+)$/i.exec(request.headers.get('authorization') ?? '')?.[1] ?? '';
    const method = request.method;

    const isWrite = method !== 'GET' && method !== 'HEAD';
    const readPerms = ['settings.company.view', 'settings.company.read', 'admin.organisation.view', 'system:admin'];
    const writePerms = ['settings.company.write', 'admin.organisation.write', 'system:admin'];

    const user = await transaction((db) =>
      requireAnyPermission(db, token, isWrite ? writePerms : readPerms, isWrite ? 'business' : false)
    );

    // GET /settings/company-profile
    if (parts.length === 2 && method === 'GET') {
      return json(await service.getCompanyProfile());
    }

    // PATCH /settings/company-profile
    if (parts.length === 2 && method === 'PATCH') {
      const payload = await request.json().catch(() => ({}));
      return json(await service.saveCompanyProfile(payload, user.id));
    }

    // Sub-resources: /settings/company-profile/<subresource>
    const sub = parts[2];
    const subId = parts[3];

    // Financial Years
    if (sub === 'financials') {
      if (method === 'POST') {
        const payload = await request.json().catch(() => ({}));
        return json(await service.saveFinancialYear(payload, user.id));
      }
      if (method === 'DELETE' && subId) {
        await service.deleteFinancialYear(subId);
        return json({ ok: true });
      }
    }

    // Licences
    if (sub === 'licences') {
      if (method === 'POST') {
        const payload = await request.json().catch(() => ({}));
        return json(await service.saveLicence(payload, user.id));
      }
      if (method === 'DELETE' && subId) {
        await service.deleteLicence(subId);
        return json({ ok: true });
      }
    }

    // Work Experience
    if (sub === 'experience') {
      if (method === 'POST') {
        const payload = await request.json().catch(() => ({}));
        return json(await service.saveWorkExperience(payload, user.id));
      }
      if (method === 'DELETE' && subId) {
        await service.deleteWorkExperience(subId);
        return json({ ok: true });
      }
    }

    // Certificates
    if (sub === 'certificates') {
      if (method === 'POST') {
        const payload = await request.json().catch(() => ({}));
        return json(await service.saveCertificate(payload, user.id));
      }
      if (method === 'DELETE' && subId) {
        await service.deleteCertificate(subId);
        return json({ ok: true });
      }
    }

    return Response.json({ detail: 'Endpoint not found' }, { status: 404, headers });
  } catch (error) {
    if (error instanceof AppError) {
      return Response.json({ detail: error.message }, { status: error.status, headers });
    }
    const msg = error instanceof Error ? error.message : 'Internal company profile error';
    return Response.json({ detail: msg }, { status: 500, headers });
  }
}
