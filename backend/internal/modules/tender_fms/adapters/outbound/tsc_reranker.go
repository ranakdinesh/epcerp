package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Version pins weights, tokenizer, quantisation and windowing, not a model alias.
const TSCRerankerModel = "minilm-l6@233902d25c440f23af6f7d6e94d2946bac0bee0a/onnx-int8-window-v1"

type TSCReranker struct {
	client           *http.Client
	pool             *pgxpool.Pool
	mu               sync.Mutex
	unavailableUntil time.Time
	timeout          time.Duration
	now              func() time.Time
}

func NewTSCReranker(pool *pgxpool.Pool) *TSCReranker {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	return &TSCReranker{pool: pool, timeout: 5 * time.Second, now: time.Now, client: &http.Client{Transport: t, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("local ranking redirect forbidden") }}}
}
func (*TSCReranker) Model() string { return TSCRerankerModel + "/" + runtime.GOARCH }
func (p *TSCReranker) Rank(ctx context.Context, q domain.TSCQuestion, passages []domain.TSCPassage) (out []domain.TSCRank, err error) {
	p.mu.Lock()
	open := p.now().Before(p.unavailableUntil)
	p.mu.Unlock()
	if open {
		return nil, errors.New("local ranking circuit open")
	}
	// Includes CPU-slot acquisition, connect, response and decoding time.
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	defer func() {
		if err != nil {
			p.mu.Lock()
			p.unavailableUntil = p.now().Add(time.Minute)
			p.mu.Unlock()
		}
	}()
	if p.pool == nil || infrastructure.DocumentSlotHeld(ctx) {
		return p.rank(ctx, q, passages)
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	worked, e := infrastructure.WithDocumentSlot(bounded, p.pool, func(c context.Context) (bool, error) { out, err = p.rank(c, q, passages); return true, err })
	if e != nil {
		return nil, e
	}
	if !worked {
		return nil, errors.New("local CPU slot busy")
	}
	return out, err
}
func (p *TSCReranker) rank(ctx context.Context, q domain.TSCQuestion, passages []domain.TSCPassage) ([]domain.TSCRank, error) {
	if len(passages) > 30 {
		return nil, errors.New("ranking candidate limit")
	}
	type candidate struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	in := struct {
		Query    string      `json:"query"`
		Passages []candidate `json:"passages"`
	}{Query: q.Text, Passages: []candidate{}}
	for _, v := range passages {
		in.Passages = append(in.Passages, candidate{v.ID, v.Text})
	}
	body, e := json.Marshal(in)
	if e != nil || len(body) > 128<<10 {
		return nil, errors.New("ranking input limit")
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// Fixed private origin; no browser-supplied endpoint or filesystem paths.
	req, e := http.NewRequestWithContext(bounded, http.MethodPost, "http://tsc-reranker:8080/rerank", bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := p.client.Do(req)
	if e != nil {
		return nil, errors.New("local ranking unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("local ranking failed")
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, 32769))
	if e != nil || len(raw) > 32768 {
		return nil, errors.New("ranking response limit")
	}
	out := struct {
		Model  string           `json:"model"`
		Scores []domain.TSCRank `json:"scores"`
	}{}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&out) != nil || d.Decode(new(any)) != io.EOF || out.Model != p.Model() {
		return nil, errors.New("invalid ranking response")
	}
	return out.Scores, nil
}
