import "server-only";
import { invalid } from "../catalogue/validation";
import { vendorID } from "../vendors/domain";
import { status, review, getAssignment, assignmentOptions, saveAssignment, reconcileApprovals } from "./service";
export const ownsVerificationPath = (path: string) => /^\/suppliers\/[^/]+\/(?:verification(?:\/assignment(?:\/options)?)?|documents\/reconcile-approvals)$/.test(path);
export async function routeVerification(request: Request, path: string, token: string, readJSON: (r: Request, max?: number) => Promise<unknown>) {
    if (!ownsVerificationPath(path))
        return null;
    const supplier = path.split('/')[2];
    if (!vendorID.test(supplier))
        invalid('Invalid supplier identity');
    const method = request.method;
    if (path.endsWith('/reconcile-approvals') && method === 'POST')
        return { value: await reconcileApprovals(token, supplier, await readJSON(request, 1024)) };
    if (path.endsWith('/assignment/options') && method === 'GET')
        return { value: await assignmentOptions(token) };
    if (path.endsWith('/assignment')) {
        if (method === 'GET')
            return { value: await getAssignment(token, supplier) };
        if (method === 'PUT')
            return { value: await saveAssignment(token, supplier, await readJSON(request, 512 * 1024)) };
    }
    if (path.endsWith('/verification')) {
        if (method === 'GET')
            return { value: await status(token, supplier) };
        if (method === 'POST')
            return { value: await review(token, supplier, await readJSON(request, 128 * 1024)) };
    }
    return { value: { detail: 'Not found' }, status: 404 };
}
