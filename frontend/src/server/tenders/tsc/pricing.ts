import type { Price, Provider, Usage } from './types';
const sourceGemini = 'https://ai.google.dev/gemini-api/docs/pricing';
export function price(provider: Provider, strong = false, now = new Date()): Price {
    const fx = Number(process.env.EPCERP_TSC_USD_INR ?? '95');
    if (!Number.isFinite(fx) || fx < 50 || fx > 200)
        throw Error('Configure a valid USD/INR estimate');
    const future = now >= new Date('2027-01-01T00:00:00Z');
    const gemini = strong ? { model: 'gemini-3.8-flash', input_usd: future ? 1.5 : .75, output_usd: future ? 7.5 : 3.75, cached_usd: future ? .15 : .075, write_usd: future ? 1.5 : .75 } : { model: 'gemini-3.1-flash-lite', input_usd: .25, output_usd: 1.5, cached_usd: .025, write_usd: .25 };
    return { ...(provider === 'gemini' ? gemini : { model: 'gpt-6-luna', input_usd: .1, output_usd: .5, cached_usd: .01, write_usd: .125 }), provider, version: '2026-11-04.standard.v1', usd_inr: fx, fx_date: process.env.EPCERP_TSC_FX_DATE ?? '2026-11-04', tier: provider === 'gemini' && process.env.EPCERP_TSC_GEMINI_TIER === 'free' ? 'free' : 'paid', source: provider === 'gemini' ? sourceGemini : 'https://developers.openai.com/api/docs/models/gpt-6-luna', max_input_tokens: provider === 'gemini' ? 1048576 : 1050000, max_output_tokens: 24576 };
}
export function estimateMicroINR(p: Price, u: Usage) {
    for (const n of Object.values(u))
        if (!Number.isSafeInteger(n) || n < 0 || n > 10000000)
            throw Error('Invalid provider token usage');
    if (u.cached_tokens + u.cache_write_tokens > u.input_tokens)
        throw Error('Invalid cached usage');
    if (p.tier === 'free')
        return 0;
    const long = p.provider === 'openai' && u.input_tokens > 272000;
    const inputFactor = long ? 2 : 1, outputFactor = long ? 1.5 : 1;
    // USD/million tokens × INR/USD gives micro-INR per token; avoid per-call paise rounding.
    return Math.ceil(((u.input_tokens - u.cached_tokens - u.cache_write_tokens) * p.input_usd * inputFactor + u.cached_tokens * p.cached_usd * inputFactor + u.cache_write_tokens * p.write_usd * inputFactor + (u.output_tokens + u.thought_tokens) * p.output_usd * outputFactor) * p.usd_inr);
}
export function reservePaise(p: Price) { return Math.max(1, Math.ceil(estimateMicroINR(p, { input_tokens: p.max_input_tokens, output_tokens: p.max_output_tokens, cached_tokens: 0, cache_write_tokens: 0, thought_tokens: 0 }) / 10000)); }
export const tenderLimit = () => bounded('EPCERP_TSC_TENDER_LIMIT_PAISE', 10000, 100, 10000);
export const dailyLimit = () => bounded('EPCERP_TSC_DAILY_LIMIT_PAISE', 50000, 100, 1000000);
function bounded(name: string, fallback: number, min: number, max: number) { const value = Number(process.env[name] ?? fallback); if (!Number.isSafeInteger(value) || value < min || value > max)
    throw Error('Invalid processing budget'); return value; }
