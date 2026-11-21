import { readFile } from 'node:fs/promises';
import { asset, createPDF } from '../documents/pdf';
import { totals } from './content';
import wording from './wording.json';
import type { Content, Item } from './types';
export function itemDetails(i: Item) { const seen = new Set<string>(), out: string[] = []; for (const raw of [...i.Description.replace(/\r\n/g, '\n').split(' · ').join('\n').split('\n'), ...i.Technical.split('\n'), ...(i.Make ? ['Make: ' + i.Make] : [])]) {
    const part = raw.trim(), key = part.toLowerCase().replace(/\s+/g, ' ');
    if (!key || i.Make && key === i.Make.trim().toLowerCase() || seen.has(key))
        continue;
    seen.add(key);
    out.push(part);
} return out; }
export function words(n: bigint): string { const ones = ['Zero', 'One', 'Two', 'Three', 'Four', 'Five', 'Six', 'Seven', 'Eight', 'Nine', 'Ten', 'Eleven', 'Twelve', 'Thirteen', 'Fourteen', 'Fifteen', 'Sixteen', 'Seventeen', 'Eighteen', 'Nineteen'], tens = ['', '', 'Twenty', 'Thirty', 'Forty', 'Fifty', 'Sixty', 'Seventy', 'Eighty', 'Ninety']; if (n < BigInt(20))
    return ones[Number(n)]; if (n < BigInt(100))
    return tens[Number(n / BigInt(10))] + (n % BigInt(10) ? ' ' + ones[Number(n % BigInt(10))] : ''); for (const [value, label] of [['10000000', 'Crore'], ['100000', 'Lakh'], ['1000', 'Thousand'], ['100', 'Hundred']]) {
    const v = BigInt(value);
    if (n >= v)
        return words(n / v) + ' ' + label + (n % v ? ' ' + words(n % v) : '');
} return ''; }
export async function render(c: Content, version: number) {
    const t = totals(c), letterhead = await readFile(asset('loi-documents/assets/letterhead.jpg'));
    return createPDF({ title: 'LETTER OF INTENT', subtitle: c.Project, letterhead, reference: c.reference, date: c.date.split('-').reverse().join('/'), footer: c.ProjectNumber + ' | Version ' + version, notice: wording.notice }, p => {
        p.paragraph('To,');
        p.paragraph(c.Vendor, true, 11);
        for (const line of [c.Address, c.ContactPerson ? 'Contact person: ' + c.ContactPerson : '', c.Phone ? 'Mobile: ' + c.Phone : '', c.Email ? 'Email: ' + c.Email : '', c.GST ? 'GST No.: ' + c.GST : ''])
            if (line)
                p.paragraph(line, false, 9.5);
        p.paragraph('Name of project: ' + c.Project, true);
        p.paragraph('Client: ' + c.Client + ' | Group: ' + c.Group, false, 9.5);
        p.paragraph('LETTER OF INTENT', true, 15);
        p.paragraph('Quotation: ' + c.Quote + ' | Currency: ' + c.Currency, false, 9);
        p.table([{ title: 'S. No.', width: 31 }, { title: 'Description / make', width: 230 }, { title: 'Qty.', width: 48 }, { title: 'Unit', width: 35 }, { title: 'Rate', width: 79 }, { title: 'Basic amount', width: 92 }], c.Items.map((i, n) => [String(n + 1), itemDetails(i).map(v => '• ' + v).join('\n'), i.Quantity, i.UOM, i.Price.pricing_mode === 'line_total' ? 'Line-total quote' : c.Currency + ' ' + i.Price.unit_price, c.Currency + ' ' + i.Price.base]));
        for (const [label, key] of [['Sub-total (basic)', 'base'], ['Discount', 'discount'], ['GST / tax', 'tax'], ['Freight / other charges', 'charges'], ['Grand total', 'total']] as const)
            p.paragraph(label + ': ' + c.Currency + ' ' + t[key], key === 'total', 9.5);
        if (c.Currency === 'INR') {
            const [w, f] = t.total.split('.');
            p.paragraph('Amount in words: Rupees ' + words(BigInt(w)) + (BigInt(f) ? ' and ' + words(BigInt(f)) + ' Paise' : '') + ' Only.', false, 9);
        }
        p.paragraph('DETAILED PURCHASE ORDER SHALL BE RELEASED ONLY AFTER CLIENT APPROVAL AND THE REQUIRED CLIENT STAMP / SIGN ON THE DRAFT PO.', true, 9.5);
        p.page();
        p.heading('Complete item scope & technical specifications');
        for (const [i, item] of c.Items.entries()) {
            p.paragraph('Item ' + (i + 1) + ' - ' + item.Description.split(' · ')[0], true);
            p.paragraph('Make: ' + item.Make + ' | Quantity: ' + item.Quantity + ' ' + item.UOM, false, 9.5);
            for (const detail of itemDetails(item))
                p.paragraph('• ' + detail, false, 9);
        }
        p.heading('Technical documents required');
        for (const [i, line] of (wording.documents[c.format as keyof typeof wording.documents] ?? []).entries())
            p.paragraph((i + 1) + '. ' + line);
        p.page();
        p.heading('Important notes');
        for (const [i, line] of wording.notes.entries())
            p.paragraph((i + 1) + '. ' + line);
        if (c.notes) {
            p.heading('Project-specific technical / inspection notes');
            p.paragraph(c.notes);
        }
        p.heading('Approved commercial terms');
        if (c.ApprovedTerms)
            p.paragraph(c.ApprovedTerms);
        for (const [i, item] of c.Items.entries()) {
            p.paragraph('Item ' + (i + 1) + ': ' + item.Description.split(' · ')[0], true, 9.5);
            p.paragraph('Delivery: ' + item.DeliveryDays + ' days, as quoted. Payment: ' + item.Payment + '. Warranty / guarantee: ' + item.Warranty + '.', false, 9.5);
        }
        p.page();
        p.heading('Terms & conditions');
        for (const [i, line] of wording.terms.entries())
            p.paragraph((i + 1) + '. ' + line);
        p.space(85);
        p.y += 15;
        p.paragraph('For Vishal Technopower Private Limited', true);
        p.y += 20;
        p.paragraph('Authorized signatory');
    });
}
