import 'server-only';
import PDFDocument from 'pdfkit';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
export const asset = (name: string) => join(process.cwd(), 'src/server/purchasing', name);
export type Column = {
    title: string;
    width: number;
};
export type PDFOptions = {
    title: string;
    subtitle: string;
    stamp?: string;
    landscape?: boolean;
    letterhead?: Buffer;
    notice?: string;
    reference?: string;
    date?: string;
    footer?: string;
};
export async function createPDF(options: PDFOptions, write: (p: Report) => void) { const [regular, bold] = await Promise.all([readFile(asset('documents/assets/Go-Regular.ttf')), readFile(asset('documents/assets/Go-Bold.ttf'))]), doc = new PDFDocument({ font: asset('documents/assets/Go-Regular.ttf'), size: 'A4', layout: options.landscape ? 'landscape' : 'portrait', margin: 40, autoFirstPage: false, info: { Title: options.title, Author: 'Vishal Technopower Private Limited' } }), chunks: Buffer[] = [], result = new Promise<Buffer>((resolve, reject) => { doc.on('data', b => chunks.push(b)); doc.on('end', () => resolve(Buffer.concat(chunks))); doc.on('error', reject); }); doc.registerFont('body', regular).registerFont('bold', bold); const p = new Report(doc, options); try {
    p.page();
    write(p);
    doc.end();
}
catch (e) {
    doc.destroy(e as Error);
    throw e;
} return result; }
export class Report {
    y = 0;
    pages = 0;
    left = 40;
    width = 0;
    bottom = 0;
    constructor(readonly doc: PDFKit.PDFDocument, readonly options: PDFOptions) { }
    font(bold = false, size = 10) { this.doc.font(bold ? 'bold' : 'body').fontSize(size).fillColor('#19202a'); }
    page() {
        this.doc.addPage();
        this.pages++;
        this.width = this.doc.page.width - 80;
        this.bottom = this.doc.page.height - (this.options.notice ? 112 : 55);
        this.font();
        if (this.options.letterhead) {
            this.doc.image(this.options.letterhead, this.doc.page.width - 270, 20, { width: 230, height: 72 });
            this.doc.text('Ref: ' + this.options.reference, 40, 105, { width: this.width - 120 });
            this.doc.text('Date: ' + this.options.date, this.doc.page.width - 150, 105, { width: 110 });
            this.doc.moveTo(40, 132).lineTo(40 + this.width, 132).strokeColor('#3c4b5f').lineWidth(.5).stroke();
            this.y = 145;
        }
        else {
            this.font(true, 15);
            this.doc.text(this.options.title, 40, 35, { width: this.width });
            this.y = 35 + this.doc.heightOfString(this.options.title, { width: this.width }) + 8;
            this.font(false, 10);
            this.doc.text(this.options.subtitle, 40, this.y, { width: this.width });
            this.y += this.doc.heightOfString(this.options.subtitle, { width: this.width }) + 8;
            if (this.options.stamp) {
                this.font(false, 8);
                this.doc.text(this.options.stamp, 40, this.y, { width: this.width });
                this.y += 18;
            }
        }
        const margin = this.doc.page.margins.bottom;
        this.doc.page.margins.bottom = 0;
        const footer = this.doc.page.height - 30;
        this.font(false, 8);
        this.doc.text((this.options.footer ?? this.options.subtitle) + ' | Page ' + this.pages, 40, footer, { width: this.width, lineBreak: false });
        if (this.options.notice) {
            this.doc.rect(40, this.doc.page.height - 87, this.width, 49).fill('#fff ae1'.replace(' ', ''));
            this.font(false, 8.5);
            this.doc.text(this.options.notice, 47, this.doc.page.height - 81, { width: this.width - 14, height: 43 });
        }
        this.doc.page.margins.bottom = margin;
        this.font();
    }
    space(height: number) { if (this.y + height > this.bottom)
        this.page(); }
    paragraph(value: string, bold = false, size = 10) { this.font(bold, size); const lines = this.wrap(value, this.width, size, bold); for (const line of lines) {
        this.space(size + 7);
        this.font(bold, size);
        this.doc.text(line, this.left, this.y, { width: this.width, lineBreak: false });
        this.y += size + 5;
    } this.y += 5; }
    heading(value: string) { this.space(38); this.doc.rect(this.left, this.y, this.width, 25).fill('#ebf1f8'); this.font(true, 11); this.doc.text(value, this.left + 8, this.y + 6, { width: this.width - 16, lineBreak: false }); this.y += 36; }
    wrap(value: string, width: number, size = 9, bold = false) { this.font(bold, size); const result: string[] = []; for (const paragraph of value.replace(/\r/g, '').split('\n')) {
        let line = '';
        for (const word of paragraph.split(/\s+/)) {
            if (this.doc.widthOfString(line ? line + ' ' + word : word) <= width) {
                line = line ? line + ' ' + word : word;
                continue;
            }
            if (line)
                result.push(line);
            line = '';
            for (const char of word) {
                if (line && this.doc.widthOfString(line + char) > width) {
                    result.push(line);
                    line = '';
                }
                line += char;
            }
        }
        result.push(line);
    } return result; }
    table(columns: Column[], rows: string[][]) { const total = columns.reduce((n, c) => n + c.width, 0), widths = columns.map(c => c.width * this.width / total); const paint = (parts: string[][], n: number, header = false) => { const h = n * 13 + 12; let x = this.left; for (const [i, w] of widths.entries()) {
        this.doc.rect(x, this.y, w, h).fillAndStroke(header ? '#ebf1f8' : '#ffffff', '#8b98a8');
        this.font(header, 9);
        for (const [j, line] of parts[i].splice(0, n).entries())
            this.doc.text(line, x + 5, this.y + 5 + j * 13, { width: w - 10, lineBreak: false });
        x += w;
    } this.y += h; }; const header = () => { const parts = columns.map((c, i) => this.wrap(c.title, widths[i] - 10, 9, true)); paint(parts, Math.max(...parts.map(p => p.length)), true); }; this.space(65); header(); for (const row of rows) {
        const parts = columns.map((_, i) => this.wrap(row[i] ?? '', widths[i] - 10)), fresh = Math.floor((this.bottom - (this.options.letterhead ? 145 : 110) - 55) / 13);
        let remaining = Math.max(...parts.map(p => p.length));
        if (this.y + remaining * 13 + 12 > this.bottom && remaining <= fresh) {
            this.page();
            header();
        }
        while (remaining) {
            let capacity = Math.floor((this.bottom - this.y - 12) / 13);
            if (capacity < 1) {
                this.page();
                header();
                capacity = Math.floor((this.bottom - this.y - 12) / 13);
            }
            const n = Math.min(remaining, capacity);
            paint(parts, n);
            remaining -= n;
            if (remaining) {
                this.page();
                header();
            }
        }
    } this.y += 12; }
}
