import 'server-only';
export const nativeOwners = ['AUTH', 'CLIENTS', 'REFERENCES', 'CATALOGUE', 'VENDOR_PROFILES', 'VENDOR_DOCUMENTS', 'VENDOR_VERIFICATION', 'TSC', 'TENDER_INTAKE', 'TENDER_REGISTER', 'INBOX', 'PERSONAL_TASKS', 'PROJECTS', 'PROJECT_TEAM', 'PURCHASING', 'PROJECT_BUDGETS', 'PLATFORM'] as const;
for (const owner of nativeOwners)
    process.env[`EPCERP_${owner}_OWNER`] ??= 'nextjs';
export function assertNativeRuntime() {
    for (const owner of nativeOwners)
        if (process.env[`EPCERP_${owner}_OWNER`] !== 'nextjs')
            throw new Error(`EPCERP_${owner}_OWNER must be nextjs; the legacy runtime has been retired`);
}
