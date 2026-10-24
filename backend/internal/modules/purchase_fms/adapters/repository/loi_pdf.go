package repository

import (
	"context"
	_ "embed"
	"fmt"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"strconv"
	"strings"
	"time"
)

// The owner-supplied panel LOI's company letterhead, extracted without changes.
//
//go:embed loi_assets/letterhead.jpg
var loiLetterhead []byte

type LOIPDFPrinter struct{}

// Keep the complete approved wording, including narrative scope, alongside the
// structured specifications. Only split explicit separators; punctuation inside
// an engineering requirement is not a safe sentence/requirement boundary.
func loiItemDetails(item domain.LOIItem) []string {
	parts := strings.Split(strings.ReplaceAll(strings.ReplaceAll(item.Description, "\r\n", "\n"), " · ", "\n"), "\n")
	parts = append(parts, strings.Split(item.Technical, "\n")...)
	if item.Make != "" {
		parts = append(parts, "Make: "+item.Make)
	}
	seen := map[string]bool{}
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Generated catalogue descriptions append the bare make. It is retained
		// below as an explicit labelled requirement rather than an extra bullet.
		if item.Make != "" && strings.EqualFold(part, strings.TrimSpace(item.Make)) {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(part), " "))
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, part)
		}
	}
	return out
}

func loiItemDescription(item domain.LOIItem) string {
	return "• " + strings.Join(loiItemDetails(item), "\n• ")
}

type loiPDF struct {
	p       gopdf.GoPdf
	ctx     context.Context
	d       domain.LOIContent
	version int32
	y       float64
	err     error
}

const loiLeft = 40.
const loiWidth = 515.
const loiBottom = 730.
const loiNotice = "LOI is subject to approval of technical documents. Manufacturing clearance shall be provided only after approval of technical documents by the end client. Quantities are tentative and subject to the end client's confirmation."

func (r *loiPDF) font(bold bool, size float64) {
	if r.err != nil {
		return
	}
	font := "body"
	if bold {
		font = "bold"
	}
	r.err = r.p.SetFont(font, "", size)
}
func (r *loiPDF) wrap(s string, w float64, bold bool, size float64) []string {
	r.font(bold, size)
	if r.err != nil {
		return nil
	}
	out := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		if line == "" {
			out = append(out, "")
			continue
		}
		parts, e := r.p.SplitTextWithOption(line, w, &gopdf.BreakOption{Mode: gopdf.BreakModeIndicatorSensitive, BreakIndicator: ' '})
		if e != nil {
			r.err = e
			return nil
		}
		out = append(out, parts...)
	}
	return out
}
func (r *loiPDF) text(x, y, w float64, s string, bold bool, size float64) {
	r.font(bold, size)
	if r.err != nil {
		return
	}
	r.p.SetXY(x, y)
	r.err = r.p.CellWithOption(&gopdf.Rect{W: w, H: size + 4}, s, gopdf.CellOption{})
}
func (r *loiPDF) newPage() {
	if r.err != nil {
		return
	}
	if r.err = r.ctx.Err(); r.err != nil {
		return
	}
	r.p.AddPage()
	r.p.SetTextColor(25, 32, 42)
	h, e := gopdf.ImageHolderByBytes(loiLetterhead)
	if e != nil {
		r.err = e
		return
	}
	r.err = r.p.ImageByHolder(h, 325, 20, &gopdf.Rect{W: 230, H: 72})
	date, _ := time.Parse("2006-01-02", r.d.Date)
	refLines := r.wrap("Ref: "+r.d.Reference, 380, false, 10)
	for i, line := range refLines {
		r.text(loiLeft, 105+float64(i)*13, 380, line, false, 10)
	}
	r.text(445, 105, 110, "Date: "+date.Format("02/01/2006"), false, 10)
	r.p.SetStrokeColor(60, 75, 95)
	r.p.SetLineWidth(.5)
	headerBottom := 126.
	if v := 105 + float64(len(refLines))*13 + 8; v > headerBottom {
		headerBottom = v
	}
	r.p.Line(loiLeft, headerBottom, 555, headerBottom)
	r.p.SetFillColor(255, 250, 225)
	r.p.RectFromUpperLeftWithStyle(loiLeft, 755, loiWidth, 49, "F")
	for i, line := range r.wrap(loiNotice, loiWidth-14, false, 8.5) {
		r.text(loiLeft+7, 761+float64(i)*11, loiWidth-14, line, false, 8.5)
	}
	r.text(loiLeft, 814, loiWidth, fmt.Sprintf("%s  |  Version %d  |  Page %d", r.d.ProjectNumber, r.version, r.p.GetNumberOfPages()), false, 8)
	r.y = headerBottom + 14
}
func (r *loiPDF) space(h float64) {
	if r.y+h > loiBottom {
		r.newPage()
	}
}
func (r *loiPDF) paragraph(s string, bold bool, size float64) {
	for _, line := range r.wrap(s, loiWidth, bold, size) {
		r.space(size + 6)
		r.text(loiLeft, r.y, loiWidth, line, bold, size)
		r.y += size + 5
	}
	r.y += 5
}
func (r *loiPDF) heading(s string) {
	r.space(35)
	if r.err != nil {
		return
	}
	r.p.SetFillColor(235, 241, 248)
	r.p.RectFromUpperLeftWithStyle(loiLeft, r.y, loiWidth, 25, "F")
	r.text(loiLeft+8, r.y+5, loiWidth-16, s, true, 11)
	r.y += 36
}
func (r *loiPDF) tableHeader() {
	if r.err != nil {
		return
	}
	labels := []string{"S. No.", "Description / make", "Qty.", "Unit", "Rate", "Basic amount"}
	widths := []float64{31, 230, 48, 35, 79, 92}
	x := loiLeft
	r.p.SetFillColor(235, 241, 248)
	for i, label := range labels {
		r.p.SetFillColor(235, 241, 248)
		r.p.RectFromUpperLeftWithStyle(x, r.y, widths[i], 27, "DF")
		r.text(x+5, r.y+6, widths[i]-10, label, true, 8.5)
		x += widths[i]
	}
	r.y += 27
}
func (r *loiPDF) tableRow(values []string) {
	if r.err != nil {
		return
	}
	widths := []float64{31, 230, 48, 35, 79, 92}
	parts := make([][]string, 6)
	remaining := 0
	for i, v := range values {
		parts[i] = r.wrap(v, widths[i]-10, false, 9)
		if len(parts[i]) > remaining {
			remaining = len(parts[i])
		}
	}
	for remaining > 0 {
		capacity := int((loiBottom - r.y - 12) / 13)
		// Keep ordinary item rows intact; only exceptionally long descriptions
		// need continuation across pages.
		freshHeight := loiBottom - 180 - 27 - 12
		freshCapacity := int(freshHeight / 13)
		if capacity < 2 || (remaining > capacity && remaining <= freshCapacity) {
			r.newPage()
			if r.err != nil {
				return
			}
			r.tableHeader()
			capacity = int((loiBottom - r.y - 12) / 13)
		}
		n := remaining
		if n > capacity {
			n = capacity
		}
		h := float64(n)*13 + 12
		x := loiLeft
		for i, w := range widths {
			r.p.RectFromUpperLeftWithStyle(x, r.y, w, h, "D")
			take := n
			if len(parts[i]) < take {
				take = len(parts[i])
			}
			for j, line := range parts[i][:take] {
				r.text(x+5, r.y+5+float64(j)*13, w-10, line, false, 9)
			}
			parts[i] = parts[i][take:]
			x += w
		}
		r.y += h
		remaining -= n
	}
}
func loiMoney(v string) string {
	p := strings.SplitN(v, ".", 2)
	whole := p[0]
	if len(whole) > 3 {
		tail := whole[len(whole)-3:]
		head := whole[:len(whole)-3]
		groups := []string{}
		for len(head) > 2 {
			groups = append([]string{head[len(head)-2:]}, groups...)
			head = head[:len(head)-2]
		}
		if head != "" {
			groups = append([]string{head}, groups...)
		}
		whole = strings.Join(groups, ",") + "," + tail
	}
	fraction := "00"
	if len(p) > 1 {
		fraction = (p[1] + "00")[:2]
	}
	return whole + "." + fraction
}
func loiWords(n int64) string {
	ones := []string{"Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}
	tens := []string{"", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"}
	if n < 20 {
		return ones[n]
	}
	if n < 100 {
		v := tens[n/10]
		if n%10 > 0 {
			v += " " + ones[n%10]
		}
		return v
	}
	for _, u := range []struct {
		value int64
		label string
	}{{10000000, "Crore"}, {100000, "Lakh"}, {1000, "Thousand"}, {100, "Hundred"}} {
		if n >= u.value {
			v := loiWords(n/u.value) + " " + u.label
			if n%u.value > 0 {
				v += " " + loiWords(n%u.value)
			}
			return v
		}
	}
	return ""
}
func (r LOIPDFPrinter) Render(ctx context.Context, d domain.LOIContent, version int32) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	totals, e := domain.LOITotals(d)
	if e != nil {
		return nil, e
	}
	w := loiPDF{ctx: ctx, d: d, version: version}
	w.p.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	if e = w.p.AddTTFFontData("body", goregular.TTF); e != nil {
		return nil, e
	}
	if e = w.p.AddTTFFontData("bold", gobold.TTF); e != nil {
		return nil, e
	}
	w.newPage()
	w.paragraph("To,", false, 10)
	w.paragraph(d.Vendor, true, 11)
	for _, v := range []string{d.Address, "Contact person: " + d.ContactPerson, "Mobile: " + d.Phone, "Email: " + d.Email, "GST No.: " + d.GST} {
		if !strings.HasSuffix(v, ": ") && v != "" {
			w.paragraph(v, false, 9.5)
		}
	}
	w.paragraph("Name of project: "+d.Project, true, 10)
	w.paragraph("Client: "+d.Client+"  |  Group: "+d.Group, false, 9.5)
	w.paragraph("LETTER OF INTENT", true, 15)
	w.paragraph("Quotation: "+d.Quote+"  |  Currency: "+d.Currency, false, 9)
	w.space(65)
	w.tableHeader()
	for i, item := range d.Items {
		rate := item.Price.Rate
		if item.Price.Mode == "line_total" {
			rate = "Line-total quote"
		} else {
			rate = d.Currency + " " + item.Price.Rate
		}
		w.tableRow([]string{strconv.Itoa(i + 1), loiItemDescription(item), item.Quantity, item.UOM, rate, d.Currency + " " + loiMoney(item.Price.Base)})
	}
	w.space(143)
	for _, entry := range []struct{ label, key string }{{"Sub-total (basic)", "base"}, {"Discount", "discount"}, {"GST / tax", "tax"}, {"Freight / other charges", "charges"}, {"Grand total", "total"}} {
		bold := entry.key == "total"
		if bold {
			w.p.SetFillColor(235, 241, 248)
			w.p.RectFromUpperLeftWithStyle(315, w.y, 240, 25, "F")
		}
		w.text(323, w.y+5, 125, entry.label, bold, 9.5)
		w.text(448, w.y+5, 105, d.Currency+" "+loiMoney(totals[entry.key]), bold, 9.5)
		w.y += 25
	}
	w.y += 10
	if d.Currency == "INR" {
		parts := strings.SplitN(totals["total"], ".", 2)
		whole, _ := strconv.ParseInt(parts[0], 10, 64)
		paise, _ := strconv.ParseInt(parts[1], 10, 64)
		words := "Amount in words: Rupees " + loiWords(whole)
		if paise > 0 {
			words += " and " + loiWords(paise) + " Paise"
		}
		w.paragraph(words+" Only.", false, 9)
	}
	w.paragraph("DETAILED PURCHASE ORDER SHALL BE RELEASED ONLY AFTER CLIENT APPROVAL AND THE REQUIRED CLIENT STAMP / SIGN ON THE DRAFT PO.", true, 9.5)
	w.newPage()
	w.heading("Complete item scope & technical specifications")
	for i, item := range d.Items {
		details := loiItemDetails(item)
		height := 50.
		for _, detail := range details {
			height += float64(len(w.wrap("• "+detail, loiWidth, false, 9)))*14 + 5
		}
		if height < loiBottom-180 {
			w.space(height)
		} else {
			w.space(85)
		}
		w.paragraph(fmt.Sprintf("Item %d — %s", i+1, strings.SplitN(item.Description, " · ", 2)[0]), true, 10)
		w.paragraph("Make: "+item.Make+" | Quantity: "+item.Quantity+" "+item.UOM, false, 9.5)
		for _, detail := range details {
			w.paragraph("• "+detail, false, 9)
		}
	}
	w.heading("Technical documents required")
	documents := map[string][]string{
		"general": {"Technical datasheets / guaranteed technical particulars", "Applicable GA drawings and technical calculations", "Quality assurance plan and applicable test certificates"},
		"cable":   {"Cable technical datasheets / guaranteed technical particulars", "Quality assurance plan (QAP)", "Applicable BIS certificate and acceptance / routine test reports", "Test certificates for tests required by the approved QAP"},
		"panel":   {"GA drawing, single-line diagram and bill of materials", "Control wiring diagram and technical datasheets", "QAP, test certificates and calibration certificates", "Warranty certificate and other client-required inspection documents"},
		"pole":    {"GA drawing, technical datasheet and foundation drawing", "Structural and foundation design calculations", "QAP and test certificates for pole material and accessories", "Warranty letter and other relevant client-required certificates"},
	}
	for i, note := range documents[d.Format] {
		w.paragraph(fmt.Sprintf("%d. %s", i+1, note), false, 10)
	}
	w.newPage()
	w.heading("Important notes")
	for i, note := range []string{"All materials shall comply with the approved technical specifications, BOQ descriptions and documents.", "Submit applicable technical documents and QAP for the end client's review. Manufacturing clearance is separate from this LOI.", "Applicable client / third-party inspection and dispatch clearance shall follow the approved documents and agreed inspection requirements.", "Factory inspection does not replace final acceptance of materials received at site in good condition.", "Any change in quantity, make or technical specification requires written approval."} {
		w.paragraph(fmt.Sprintf("%d. %s", i+1, note), false, 10)
	}
	if d.Notes != "" {
		w.heading("Project-specific technical / inspection notes")
		w.paragraph(d.Notes, false, 10)
	}
	w.heading("Approved commercial terms")
	if d.ApprovedTerms != "" {
		w.paragraph(d.ApprovedTerms, false, 10)
	}
	for i, item := range d.Items {
		w.paragraph(fmt.Sprintf("Item %d: %s", i+1, strings.SplitN(item.Description, " · ", 2)[0]), true, 9.5)
		w.paragraph(fmt.Sprintf("Delivery: %d days, as quoted. Payment: %s. Warranty / guarantee: %s.", item.DeliveryDays, item.Payment, item.Warranty), false, 9.5)
	}
	w.newPage()
	w.heading("Terms & conditions")
	terms := []string{"Price basis, delivery, payment and warranty are as recorded in the approved commercial terms above. Tax, discounts and freight are shown separately in the commercial schedule.", "Materials and workmanship shall conform to the approved technical specifications and drawings. The vendor shall provide stamped / signed technical compliance acknowledgement.", "Material shall be dispatched only after receiving the required written dispatch clearance. Inspection and testing shall follow the approved QAP and client requirements.", "Any proposed deviation in quantity, make or specification shall be submitted for written approval before implementation.", "The vendor shall indemnify Vishal Technopower Private Limited against claims or losses arising from defective supplies, technical non-compliance or legal violations related to the goods supplied.", "Both parties shall maintain the confidentiality of proprietary and technical information exchanged in connection with the order.", "Vishal Technopower Private Limited reserves the right to terminate for failure to meet agreed delivery, quality or contractual obligations with seven days' written notice. Recovery of advances and alternative procurement costs shall follow the agreed contract.", "Disputes shall first be resolved amicably. Unresolved disputes shall be referred to arbitration under the Arbitration and Conciliation Act, 1996, with the venue at Ghaziabad, Uttar Pradesh.", "The vendor shall comply with applicable statutory and regulatory requirements, including GST and environmental requirements.", "This LOI is conditional on client technical approval. It is not manufacturing clearance or a released purchase order."}
	for i, note := range terms {
		w.paragraph(fmt.Sprintf("%d. %s", i+1, note), false, 10)
	}
	w.space(85)
	w.y += 15
	w.paragraph("For Vishal Technopower Private Limited", true, 10)
	w.y += 20
	w.paragraph("Authorized signatory", false, 10)
	if w.err != nil {
		return nil, w.err
	}
	return w.p.GetBytesPdfReturnErr()
}
