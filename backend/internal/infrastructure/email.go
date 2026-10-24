package infrastructure

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

// EmailService is shared transport, not a business workflow. Modules supply
// recipient, subject and message through their own application ports.
type EmailService struct {
	key, sender, name string
	PublicURL         string
	client            *http.Client
}

func (m *EmailService) String() string   { return "EmailService{values omitted}" }
func (m *EmailService) GoString() string { return m.String() }
func NewEmailService(lookup func(string) (string, bool)) (*EmailService, error) {
	get := func(k string) string { v, _ := lookup(k); return strings.TrimSpace(v) }
	sender := get("EPCERP_EMAIL_SENDER_EMAIL")
	if sender == "" {
		sender = get("EPCERP_RECOVERY_SENDER_EMAIL")
	}
	name := get("EPCERP_EMAIL_SENDER_NAME")
	if name == "" {
		name = get("EPCERP_RECOVERY_SENDER_NAME")
	}
	if name == "" {
		name = "Vishal Technopower | epcERP"
	}
	public := get("EPCERP_PUBLIC_URL")
	if public == "" {
		public = "https://erp.vishaltechnopower.in"
	}
	u, e := url.Parse(public)
	local := e == nil && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") && (get("VTPL_ENVIRONMENT") == "development" || get("VTPL_ENVIRONMENT") == "test")
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !local) {
		return nil, errors.New("invalid EPCERP_PUBLIC_URL")
	}
	m := &EmailService{key: get("EPCERP_BREVO_API_KEY"), sender: sender, name: name, PublicURL: strings.TrimRight(public, "/"), client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if m.key == "" && sender == "" {
		return m, nil
	}
	a, e := mail.ParseAddress(sender)
	if m.key == "" || e != nil || a.Address != sender || len(name) > 100 {
		return nil, errors.New("configure Brevo API key and verified email sender together")
	}
	return m, nil
}
func (m *EmailService) Ready() bool { return m.key != "" && m.sender != "" }

type emailParagraph struct {
	Text string
	URL  string
}

var professionalEmail = template.Must(template.New("email").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body style="margin:0;background:#f1f5f9;font-family:Arial,sans-serif;color:#0f172a"><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td style="padding:32px 16px"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:600px;margin:auto;background:#fff;border:1px solid #e2e8f0;border-radius:16px"><tr><td style="padding:28px 32px;background:#1e3a8a;color:#fff;border-radius:16px 16px 0 0"><div style="font-size:20px;font-weight:bold">VISHAL TECHNOPOWER</div><div style="margin-top:6px;font-size:13px;color:#dbeafe">epcERP · Secure employee access</div></td></tr><tr><td style="padding:28px 32px"><h1 style="margin:0 0 24px;font-size:22px;line-height:1.4">{{.Subject}}</h1>{{range .Paragraphs}}{{if .URL}}<p style="margin:20px 0;overflow-wrap:anywhere"><a href="{{.URL}}" style="color:#1e3a8a;font-weight:bold">Open epcERP securely</a></p>{{else}}<p style="margin:0 0 16px;line-height:1.7;white-space:pre-wrap;overflow-wrap:anywhere">{{.Text}}</p>{{end}}{{end}}</td></tr><tr><td style="padding:20px 32px;border-top:1px solid #e2e8f0;color:#64748b;font-size:12px;line-height:1.7">This is an automated message from Vishal Technopower. Keep your credentials private. If you did not expect this email, contact your administrator.</td></tr></table></td></tr></table></body></html>`))

func RenderEmail(subject, message string) (string, error) {
	parts := []emailParagraph{}
	for _, p := range strings.Split(message, "\n\n") {
		u, e := url.Parse(strings.TrimSpace(p))
		if e == nil && u.Host != "" && u.User == nil && (u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
			parts = append(parts, emailParagraph{URL: u.String()})
		} else {
			parts = append(parts, emailParagraph{Text: p})
		}
	}
	var b bytes.Buffer
	e := professionalEmail.Execute(&b, struct {
		Subject    string
		Paragraphs []emailParagraph
	}{subject, parts})
	return b.String(), e
}
func (m *EmailService) Send(ctx context.Context, to, subject, message string) error {
	if !m.Ready() {
		return errors.New("email service not configured")
	}
	a, e := mail.ParseAddress(to)
	if e != nil || a.Address != to || len(to) > 255 || len(subject) == 0 || len(subject) > 200 || strings.ContainsAny(subject, "\r\n") || len(message) > 32768 {
		return errors.New("invalid email message")
	}
	html, e := RenderEmail(subject, message)
	if e != nil {
		return errors.New("email rendering failed")
	}
	body, e := json.Marshal(map[string]any{"sender": map[string]string{"email": m.sender, "name": m.name}, "to": []map[string]string{{"email": to}}, "subject": subject, "textContent": message, "htmlContent": html})
	if e != nil {
		return errors.New("email encoding failed")
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(body))
	if e != nil {
		return errors.New("email request failed")
	}
	req.Header.Set("api-key", m.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	response, e := m.client.Do(req)
	if e != nil {
		return errors.New("email delivery failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8192))
	if response.StatusCode != 201 {
		return errors.New("email provider rejected request")
	}
	return nil
}

// Derive a separate encryption context for queued credentials from the existing
// server-owned image key, without storing another plaintext secret.
func NewEmailCipher(baseKey string) (*DocumentCipher, error) {
	raw, e := hex.DecodeString(baseKey)
	if e != nil || len(raw) != 32 {
		return nil, errors.New("invalid email encryption master key")
	}
	h := hmac.New(sha256.New, raw)
	_, _ = h.Write([]byte("epcERP/credential-email/v1"))
	return NewDocumentCipher(hex.EncodeToString(h.Sum(nil)))
}
