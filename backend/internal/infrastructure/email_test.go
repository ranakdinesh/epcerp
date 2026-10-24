package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type emailRoundTrip func(*http.Request) (*http.Response, error)

func (f emailRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func emailSettings() map[string]string {
	return map[string]string{"EPCERP_BREVO_API_KEY": "fixture-key", "EPCERP_EMAIL_SENDER_EMAIL": "sender@example.invalid", "EPCERP_EMAIL_SENDER_NAME": "Vishal Technopower"}
}
func configuredEmail(v map[string]string) (*EmailService, error) {
	return NewEmailService(func(k string) (string, bool) { s, ok := v[k]; return s, ok })
}
func TestSharedEmailConfigurationAndProfessionalEscaping(t *testing.T) {
	m, e := configuredEmail(nil)
	if e != nil || m.Ready() || m.PublicURL != "https://erp.vishaltechnopower.in" {
		t.Fatal("disabled/default configuration", e)
	}
	for _, field := range []string{"EPCERP_BREVO_API_KEY", "EPCERP_EMAIL_SENDER_EMAIL"} {
		v := emailSettings()
		delete(v, field)
		if _, e := configuredEmail(v); e == nil {
			t.Fatal("partial mail settings accepted")
		}
	}
	for _, u := range []string{"http://remote.example.invalid", "https://user@erp.example.invalid", "https://erp.example.invalid?secret=x"} {
		v := emailSettings()
		v["EPCERP_PUBLIC_URL"] = u
		if _, e := configuredEmail(v); e == nil {
			t.Fatal("unsafe origin accepted")
		}
	}
	html, e := RenderEmail("<script>subject</script>", "Hello <script>attack</script>\n\nTemporary password: A&B<9\n\nhttps://erp.vishaltechnopower.in/login")
	if e != nil || strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") || !strings.Contains(html, "A&amp;B&lt;9") || !strings.Contains(html, `href="https://erp.vishaltechnopower.in/login"`) {
		t.Fatal("professional email unsafe or missing login link", e)
	}
}
func TestSharedBrevoRequestAndSafeErrors(t *testing.T) {
	m, e := configuredEmail(emailSettings())
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(fmt.Sprintf("%#v", m), "fixture-key") {
		t.Fatal("printable credential")
	}
	m.client.Transport = emailRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.brevo.com/v3/smtp/email" || r.Header.Get("api-key") != "fixture-key" {
			t.Fatal("wrong provider request")
		}
		var p map[string]any
		if json.NewDecoder(r.Body).Decode(&p) != nil {
			t.Fatal("invalid message")
		}
		if p["textContent"] != "Hello" || !strings.Contains(p["htmlContent"].(string), "VISHAL TECHNOPOWER") {
			t.Fatal("plain text or HTML alternative missing")
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	if e = m.Send(context.Background(), "person@example.invalid", "Welcome", "Hello"); e != nil {
		t.Fatal(e)
	}
	m.client.Transport = emailRoundTrip(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("secret-key/person@example.invalid")
	})
	if e = m.Send(context.Background(), "person@example.invalid", "Welcome", "Hello"); e == nil || strings.Contains(e.Error(), "secret-key") || strings.Contains(e.Error(), "example.invalid") {
		t.Fatal("provider detail leaked")
	}
}
