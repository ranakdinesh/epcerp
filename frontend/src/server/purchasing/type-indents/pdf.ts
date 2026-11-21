import 'server-only';
import { indentApprovalLabels, indentFields, indentSignatureNotice, readIndentContent, type IndentRun } from '@/lib/indent-content';
import PDFDocument from 'pdfkit';
import { readFile } from 'node:fs/promises';
import { asset, Report } from '../documents/pdf';

type DraftPrint = {
    category: string;
    project: string;
    client: string;
    site: string;
    date: Date;
    terms: string;
    items: { description: string; quantity: string; unit: string }[];
};

export async function renderIndentDraft(model: DraftPrint) {
    const content = readIndentContent(model.terms);
    const cable = model.category === 'CAB';
    const logo = await readFile(asset(cable ? 'documents/assets/indent-cable-logo.jpg' : 'documents/assets/indent-logo.png'));
    const [regular, bold] = await Promise.all([
        readFile(asset('documents/assets/LiberationSerif-Regular.ttf')),
        readFile(asset('documents/assets/LiberationSerif-Bold.ttf')),
    ]);
    const doc = new PDFDocument({ size: 'A4', margin: 38, autoFirstPage: false, font: asset('documents/assets/LiberationSerif-Regular.ttf'), info: { Title: 'Purchase Indent' } });
    doc.registerFont('indent-regular', regular).registerFont('indent-bold', bold);
    const chunks: Buffer[] = [];
    const result = new Promise<Buffer>((resolve, reject) => {
        doc.on('data', chunk => chunks.push(chunk));
        doc.on('end', () => resolve(Buffer.concat(chunks)));
        doc.on('error', reject);
    });
    const categoryTitle = ({ CAB: 'Cable', PAN: 'Panel', LED: '' } as Record<string, string>)[model.category] ?? model.category;
    const title = cable ? 'PURCHASE INDENT\n(Cable)' : categoryTitle ? `PURCHASE INDENT  (${categoryTitle})` : 'PURCHASE INDENT';
    const date = new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Kolkata', dateStyle: 'short' }).format(model.date);
    class IndentPage extends Report {
        font(bold = false, size = 10) {
            doc.font(bold ? 'indent-bold' : 'indent-regular').fontSize(size).fillColor('#000000');
        }
        page() {
            doc.addPage();
            this.pages++;
            this.left = cable ? 48 : 38;
            this.width = doc.page.width - this.left * 2;
            this.bottom = doc.page.height - (cable ? 108 : 87);
            const logoWidth = cable ? 244 : 294;
            doc.image(logo, doc.page.width - this.left - logoWidth, cable ? 15 : 34, { width: logoWidth });
            this.font(true, 10);
            if (this.pages === 1) {
                doc.text(`Date: ${date}`, this.left, cable ? 108 : 145, { width: this.width, align: 'right' });
                this.font(true, 15);
                doc.text(title, this.left, cable ? 152 : 176, { width: this.width, align: 'center' });
                this.y = doc.y + 5;
            } else {
                this.y = cable ? 112 : 142;
            }
            if (cable) {
                const margin = doc.page.margins.bottom;
                doc.page.margins.bottom = 0;
                this.font(true, 8);
                doc.text('B-4, Hans Plaza, Ambedkar Road, Ghaziabad, U.P. - 201001 | E-mail: vishaltechnopower@yahoo.com',
                    this.left, doc.page.height - 43, { width: this.width, align: 'center' });
                doc.text('T: +91 - 120 - 4122684 | Website: http://www.vishaltechnopower.com', { width: this.width, align: 'center' });
                doc.page.margins.bottom = margin;
            }
        }
    }
    const page = new IndentPage(doc, { title, subtitle: model.project });
    try {
        page.page();
        const box = [`${cable ? 'Name Of Work' : 'Name of Project'}: ${model.project}`, model.client && `Client: ${model.client}`, model.site && `Delivery site: ${model.site}`].filter(Boolean).join('\n');
        page.font(false, 10);
        const boxHeight = doc.heightOfString(box, { width: page.width - 10 }) + 10;
        doc.rect(page.left, page.y, page.width, boxHeight).lineWidth(0.5).stroke();
        doc.text(box, page.left + 5, page.y + 5, { width: page.width - 10 });
        page.y += boxHeight + (cable ? 0 : 18);

        const widths = [43, page.width - 151, 54, 54];
        const tail = cable ? ['Unit', 'Qty.'] : ['Qty.', 'Unit'];
        function cells(lines: string[][], count: number, bold: boolean) {
            let x = page.left;
            const height = count * 11 + 6;
            for (const [index, width] of widths.entries()) {
                doc.rect(x, page.y, width, height).stroke();
                page.font(bold, 9);
                for (const [line, value] of lines[index].splice(0, count).entries()) {
                    doc.text(value, x + 4, page.y + 3 + line * 11, { width: width - 8, lineBreak: false, align: index === 1 && !bold ? 'left' : 'center' });
                }
                x += width;
            }
            page.y += height;
        }
        const header = () => cells(['S. No.', 'Description', ...tail].map(text => [text]), 1, true);
        function richFont(run: IndentRun) {
            page.font(run.bold, 10);
            if (run.italic) doc.font(run.bold ? 'Times-BoldItalic' : 'Times-Italic');
        }
        function wrapRich(paragraphs: IndentRun[][]) {
            const maxWidth = page.width - 16;
            return paragraphs.flatMap(paragraph => {
                const lines: IndentRun[][] = [[]];
                let width = 0;
                for (const run of paragraph) {
                    richFont(run);
                    for (const word of run.text.split(/(\s+)/).filter(Boolean)) {
                        const pieces = doc.widthOfString(word) > maxWidth ? Array.from(word) : [word];
                        for (const text of pieces) {
                            const pieceWidth = doc.widthOfString(text);
                            if (width + pieceWidth > maxWidth) {
                                lines.push([]);
                                width = 0;
                            }
                            if (!width && !text.trim()) continue;
                            lines.at(-1)!.push({ ...run, text });
                            width += pieceWidth;
                        }
                    }
                }
                return lines;
            });
        }
        function richSection(title: string, paragraphs: IndentRun[][], spacing = 1, repeatHeader: () => void = () => {}) {
            const lineHeight = 14 * spacing;
            const lines = wrapRich(paragraphs);
            const headingHeight = 24;
            while (lines.length) {
                if (page.y + headingHeight + lineHeight > page.bottom) {
                    page.page();
                    repeatHeader();
                }
                const capacity = Math.floor((page.bottom - page.y - headingHeight) / lineHeight);
                const portion = lines.splice(0, capacity);
                const height = headingHeight + portion.length * lineHeight;
                doc.rect(page.left, page.y, page.width, height).stroke();
                page.font(true, 10);
                doc.text(title, page.left + 8, page.y + 6, { lineBreak: false });
                portion.forEach((line, index) => {
                    let x = page.left + 8;
                    for (const run of line) {
                        richFont(run);
                        doc.text(run.text, x, page.y + 19 + index * lineHeight, { lineBreak: false, underline: run.underline });
                        x += doc.widthOfString(run.text);
                    }
                });
                page.y += height;
            }
        }
        page.space(55);
        header();
        richSection(indentFields[0][1], content.specifications, content.spacing?.specifications, header);
        const tableRows = model.items.map((item, index) => ({ ...item, number: String(index + 1) }));
        for (const item of tableRows) {
            const description = item.description;
            const values = [item.number, description, ...(cable ? [item.unit, item.quantity] : [item.quantity, item.unit])];
            const lines = values.map((value, i) => page.wrap(value, widths[i] - 8, 9));
            while (lines.some(part => part.length)) {
                let capacity = Math.floor((page.bottom - page.y - 6) / 11);
                if (capacity < 1) {
                    page.page();
                    header();
                    capacity = Math.floor((page.bottom - page.y - 6) / 11);
                }
                cells(lines, Math.min(capacity, Math.max(...lines.map(part => part.length))), false);
            }
        }
        for (const [key, title] of indentFields.slice(1)) {
            if (!content[key].some(paragraph => paragraph.some(run => run.text.trim()))) continue;
            page.y += 12;
            richSection(title, content[key], content.spacing?.[key]);
        }
        const footerY = page.bottom + 10;
        page.font(true, 10);
        const [raised, checked, approved] = indentApprovalLabels;
        doc.text(raised, page.left, footerY, { lineBreak: false });
        doc.text(checked, page.left + (page.width - doc.widthOfString(checked)) / 2, footerY, { lineBreak: false });
        doc.text(approved, page.left + page.width - doc.widthOfString(approved), footerY, { lineBreak: false });
        page.font(false, 9);
        doc.text(indentSignatureNotice, page.left + (page.width - doc.widthOfString(indentSignatureNotice)) / 2,
            footerY + 18, { lineBreak: false });
        doc.end();
        return await result;
    } catch (error) {
        doc.destroy();
        throw error;
    }
}
