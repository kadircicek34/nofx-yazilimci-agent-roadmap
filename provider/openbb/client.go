package openbb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type Config struct {
	PythonBinary string
	ScriptPath   string
	Timeout      time.Duration
	CacheTTL     time.Duration
	NewsLimit    int
	HistoryDays  int
	MacroSymbols []string
	IncludeMacro bool
	IncludeNews  bool
	IncludeTech  bool
}

type Client struct {
	pythonBinary string
	scriptPath   string
	timeout      time.Duration
	cacheTTL     time.Duration
	defaultReq   EnrichmentRequest

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	expiresAt time.Time
	data      *EnrichmentResponse
}

type EnrichmentRequest struct {
	Symbols          []string `json:"symbols,omitempty"`
	MacroSymbols     []string `json:"macro_symbols,omitempty"`
	NewsLimit        int      `json:"news_limit,omitempty"`
	HistoryDays      int      `json:"history_days,omitempty"`
	TimeoutSeconds   int      `json:"timeout_seconds,omitempty"`
	IncludeMacro     bool     `json:"include_macro"`
	IncludeNews      bool     `json:"include_news"`
	IncludeTechnical bool     `json:"include_technical"`
}

type EnrichmentResponse struct {
	GeneratedAt string                    `json:"generated_at,omitempty"`
	Macro       *MacroContext             `json:"macro,omitempty"`
	Symbols     map[string]*SymbolContext `json:"symbols,omitempty"`
	Errors      []string                  `json:"errors,omitempty"`
}

type MacroContext struct {
	Summary       string               `json:"summary,omitempty"`
	Benchmarks    []MacroAssetContext  `json:"benchmarks,omitempty"`
	InterestRates *InterestRateContext `json:"interest_rates,omitempty"`
}

type MacroAssetContext struct {
	Symbol          string            `json:"symbol,omitempty"`
	LookupSymbol    string            `json:"lookup_symbol,omitempty"`
	LastPrice       float64           `json:"last_price,omitempty"`
	Return1D        float64           `json:"return_1d,omitempty"`
	Return5D        float64           `json:"return_5d,omitempty"`
	Return20D       float64           `json:"return_20d,omitempty"`
	Trend           string            `json:"trend,omitempty"`
	Sentiment       *SentimentSummary `json:"sentiment,omitempty"`
	TopHeadlines    []NewsHeadline    `json:"top_headlines,omitempty"`
	HeadlineSummary string            `json:"headline_summary,omitempty"`
}

type InterestRateContext struct {
	Country  string  `json:"country,omitempty"`
	Latest   float64 `json:"latest,omitempty"`
	Previous float64 `json:"previous,omitempty"`
	Delta    float64 `json:"delta,omitempty"`
	AsOf     string  `json:"as_of,omitempty"`
}

type SymbolContext struct {
	InputSymbol     string            `json:"input_symbol,omitempty"`
	LookupSymbol    string            `json:"lookup_symbol,omitempty"`
	Technical       *TechnicalSummary `json:"technical,omitempty"`
	Sentiment       *SentimentSummary `json:"sentiment,omitempty"`
	TopHeadlines    []NewsHeadline    `json:"top_headlines,omitempty"`
	HeadlineSummary string            `json:"headline_summary,omitempty"`
}

type TechnicalSummary struct {
	Close         float64 `json:"close,omitempty"`
	Return1D      float64 `json:"return_1d,omitempty"`
	Return5D      float64 `json:"return_5d,omitempty"`
	Return20D     float64 `json:"return_20d,omitempty"`
	SMA20         float64 `json:"sma20,omitempty"`
	SMA50         float64 `json:"sma50,omitempty"`
	EMA20         float64 `json:"ema20,omitempty"`
	RSI14         float64 `json:"rsi14,omitempty"`
	MACD          float64 `json:"macd,omitempty"`
	MACDSignal    float64 `json:"macd_signal,omitempty"`
	MACDHist      float64 `json:"macd_hist,omitempty"`
	Volatility20D float64 `json:"volatility_20d,omitempty"`
	Trend         string  `json:"trend,omitempty"`
	Momentum      string  `json:"momentum,omitempty"`
}

type SentimentSummary struct {
	Label        string  `json:"label,omitempty"`
	Score        float64 `json:"score,omitempty"`
	BullishCount int     `json:"bullish_count,omitempty"`
	BearishCount int     `json:"bearish_count,omitempty"`
	NeutralCount int     `json:"neutral_count,omitempty"`
}

type NewsHeadline struct {
	Date      string  `json:"date,omitempty"`
	Title     string  `json:"title,omitempty"`
	Source    string  `json:"source,omitempty"`
	Summary   string  `json:"summary,omitempty"`
	URL       string  `json:"url,omitempty"`
	Sentiment string  `json:"sentiment,omitempty"`
	Score     float64 `json:"score,omitempty"`
}

func NewClient(cfg Config) *Client {
	if cfg.PythonBinary == "" {
		cfg.PythonBinary = "python3"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 45 * time.Second
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 15 * time.Minute
	}
	if cfg.NewsLimit <= 0 {
		cfg.NewsLimit = 4
	}
	if cfg.HistoryDays <= 0 {
		cfg.HistoryDays = 90
	}
	if len(cfg.MacroSymbols) == 0 {
		cfg.MacroSymbols = []string{"SPY", "QQQ", "DX-Y.NYB", "GLD", "CL=F", "^VIX"}
	}
	if cfg.ScriptPath == "" {
		cfg.ScriptPath = defaultScriptPath()
	}

	return &Client{
		pythonBinary: cfg.PythonBinary,
		scriptPath:   cfg.ScriptPath,
		timeout:      cfg.Timeout,
		cacheTTL:     cfg.CacheTTL,
		defaultReq: EnrichmentRequest{
			NewsLimit:        cfg.NewsLimit,
			HistoryDays:      cfg.HistoryDays,
			MacroSymbols:     cfg.MacroSymbols,
			IncludeMacro:     cfg.IncludeMacro,
			IncludeNews:      cfg.IncludeNews,
			IncludeTechnical: cfg.IncludeTech,
		},
		cache: make(map[string]cacheEntry),
	}
}

func defaultScriptPath() string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "provider/openbb/scripts/enrich.py"
	}
	return filepath.Join(filepath.Dir(currentFile), "scripts", "enrich.py")
}

func (c *Client) Enrich(ctx context.Context, req EnrichmentRequest) (*EnrichmentResponse, error) {
	merged := c.defaultReq
	if len(req.Symbols) > 0 {
		merged.Symbols = req.Symbols
	}
	if len(req.MacroSymbols) > 0 {
		merged.MacroSymbols = req.MacroSymbols
	}
	if req.NewsLimit > 0 {
		merged.NewsLimit = req.NewsLimit
	}
	if req.HistoryDays > 0 {
		merged.HistoryDays = req.HistoryDays
	}
	if req.TimeoutSeconds > 0 {
		merged.TimeoutSeconds = req.TimeoutSeconds
	}
	merged.IncludeMacro = req.IncludeMacro || merged.IncludeMacro
	merged.IncludeNews = req.IncludeNews || merged.IncludeNews
	merged.IncludeTechnical = req.IncludeTechnical || merged.IncludeTechnical

	merged.Symbols = normalizeStrings(merged.Symbols)
	merged.MacroSymbols = normalizeStrings(merged.MacroSymbols)
	if len(merged.Symbols) == 0 && !merged.IncludeMacro {
		return nil, nil
	}

	cacheKey := merged.cacheKey()
	if cached, ok := c.getCache(cacheKey); ok {
		return cached, nil
	}

	payload, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenBB request: %w", err)
	}

	timeout := c.timeout
	if merged.TimeoutSeconds > 0 {
		timeout = time.Duration(merged.TimeoutSeconds) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, c.pythonBinary, c.scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("OpenBB enrichment timed out after %s", timeout)
		}
		if strings.TrimSpace(stderr.String()) != "" {
			return nil, fmt.Errorf("OpenBB enrichment failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("OpenBB enrichment failed: %w", err)
	}

	var resp EnrichmentResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("decode OpenBB enrichment response: %w", err)
	}

	c.setCache(cacheKey, &resp)
	return &resp, nil
}

func (r EnrichmentRequest) cacheKey() string {
	copyReq := r
	copyReq.Symbols = normalizeStrings(copyReq.Symbols)
	copyReq.MacroSymbols = normalizeStrings(copyReq.MacroSymbols)
	b, _ := json.Marshal(copyReq)
	return string(b)
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToUpper(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (c *Client) getCache(key string) (*EnrichmentResponse, bool) {
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.cache, key)
			c.mu.Unlock()
		}
		return nil, false
	}
	return entry.data, true
}

func (c *Client) setCache(key string, data *EnrichmentResponse) {
	if data == nil {
		return
	}
	c.mu.Lock()
	c.cache[key] = cacheEntry{expiresAt: time.Now().Add(c.cacheTTL), data: data}
	c.mu.Unlock()
}
