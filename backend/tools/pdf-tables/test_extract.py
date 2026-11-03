"""Synthetic layout regressions: never contains customer tender attachments."""
import tempfile
import unittest
from pathlib import Path
from extract import extract


def pdf(commands):
    stream = commands.encode('ascii')
    objects = [b'<< /Type /Catalog /Pages 2 0 R >>', b'<< /Type /Pages /Kids [3 0 R] /Count 1 >>', b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 600 800] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>', b'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>', b'<< /Length '+str(len(stream)).encode()+b' >>\nstream\n'+stream+b'\nendstream']
    data = bytearray(b'%PDF-1.4\n'); offsets = [0]
    for i, obj in enumerate(objects, 1):
        offsets.append(len(data)); data.extend(str(i).encode()+b' 0 obj\n'+obj+b'\nendobj\n')
    start = len(data); data.extend(b'xref\n0 6\n0000000000 65535 f \n')
    for offset in offsets[1:]: data.extend(f'{offset:010} 00000 n \n'.encode())
    data.extend(f'trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n{start}\n%%EOF'.encode())
    return data


def text(x, y, value):
    return f'BT /F1 10 Tf {x} {y} Td ({value}) Tj ET\n'


class TableLayouts(unittest.TestCase):
    def read(self, commands):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder)/'synthetic.pdf'; path.write_bytes(pdf(commands))
            return extract(path)['pages'][0]

    def test_centered_label_preserves_wrapped_scope(self):
        commands = '40 600 520 120 re S\n200 600 m 200 720 l S\n'
        commands += text(45, 655, 'Name of Work')
        for y, value in [(700,'First scope line'),(680,'Middle scope line'),(660,'Final scope line')]: commands += text(205,y,value)
        cells = self.read(commands)['tables'][0]['cells']
        self.assertEqual(len(cells),2)
        self.assertEqual(cells[0]['text'],'Name of Work')
        self.assertEqual(cells[1]['text'],'First scope line\nMiddle scope line\nFinal scope line')
        self.assertEqual(cells[0]['row'],cells[1]['row'])

    def test_two_pairs_headers_and_merged_value(self):
        commands = '40 500 520 200 re S\n'
        for y in [650,600]: commands += f'40 {y} m 560 {y} l S\n'
        for x in [170,300,430]: commands += f'{x} 600 m {x} 700 l S\n'
        commands += '170 500 m 170 600 l S\n'
        for x,value in [(45,'Description'),(175,'Amount'),(305,'Description'),(435,'Unit')]: commands += text(x,680,value)
        for x,value in [(45,'EMD (Rs.)'),(175,'2500'),(305,'Validity'),(435,'60 days')]: commands += text(x,625,value)
        commands += text(45,550,'Name of Work')+text(175,550,'Merged full scope')
        cells = self.read(commands)['tables'][0]['cells']
        emd = next(c for c in cells if c['text']=='2500')
        self.assertEqual(emd['column_header'],'Amount')
        self.assertEqual(emd['column'],1)
        merged = next(c for c in cells if c['text']=='Merged full scope')
        self.assertEqual(merged['column_span'],3)
        self.assertEqual(merged['row'],2)

    def test_continuing_border_is_clipped_to_visible_page(self):
        page = self.read('40 750 520 100 re S\n200 750 m 200 850 l S\n'+text(45,770,'Visible label')+text(205,770,'Visible value'))
        cells = page['tables'][0]['cells']
        self.assertEqual(len(cells),2)
        for c in cells:
            self.assertGreaterEqual(c['y'],0)
            self.assertLessEqual(c['y']+c['height'],page['height'])

    def test_borderless_text_is_not_invented_as_grid(self):
        page = self.read(text(40,700,'EMD')+text(300,700,'2500'))
        self.assertEqual(page['tables'],[])


if __name__ == '__main__': unittest.main()
