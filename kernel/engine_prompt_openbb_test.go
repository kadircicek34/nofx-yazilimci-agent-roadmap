package kernel

import (
	"strings"
	"testing"
	"time"

	"nofx/market"
	openbbprovider "nofx/provider/openbb"
	"nofx/store"
)

func TestBuildUserPrompt_IncludesOpenBBSections(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&cfg)
	ctx := createTestContext()
	ctx.MarketDataMap = map[string]*market.Data{
		"PIPPINUSDT": {Symbol: "PIPPINUSDT", CurrentPrice: 0.4937},
		"BTCUSDT":    {Symbol: "BTCUSDT", CurrentPrice: 85000, PriceChange1h: 1.2, PriceChange4h: 3.4},
		"ETHUSDT":    {Symbol: "ETHUSDT", CurrentPrice: 4500},
	}
	ctx.OpenBBData = &openbbprovider.EnrichmentResponse{
		Macro: &openbbprovider.MacroContext{
			Summary: "SPY risk-on, DXY cooling, BTC benchmark firm",
			Benchmarks: []openbbprovider.MacroAssetContext{{
				Symbol:       "SPY",
				LookupSymbol: "SPY",
				LastPrice:    510.2,
				Return1D:     0.8,
				Return5D:     1.9,
				Return20D:    4.2,
				Trend:        "bullish",
				Sentiment:    &openbbprovider.SentimentSummary{Label: "bullish", Score: 0.42},
				TopHeadlines: []openbbprovider.NewsHeadline{{Title: "Stocks climb into the close", Source: "Reuters", Sentiment: "bullish"}},
			}},
			InterestRates: &openbbprovider.InterestRateContext{Country: "united_states", Latest: 3.66, Previous: 3.70, Delta: -0.04, AsOf: time.Now().UTC().Format(time.RFC3339)},
		},
		Symbols: map[string]*openbbprovider.SymbolContext{
			"PIPPINUSDT": {
				LookupSymbol:    "PIPPIN-USD",
				Technical:       &openbbprovider.TechnicalSummary{Close: 0.4937, Return1D: 2.2, Return5D: 4.4, Return20D: 9.1, RSI14: 61.3, MACD: 0.0123, MACDSignal: 0.0101, MACDHist: 0.0022, SMA20: 0.46, SMA50: 0.42, EMA20: 0.47, Volatility20D: 58.1, Trend: "bullish", Momentum: "positive"},
				Sentiment:       &openbbprovider.SentimentSummary{Label: "bullish", Score: 0.31, BullishCount: 2, BearishCount: 0, NeutralCount: 1},
				HeadlineSummary: "Treasury firms expand Solana exposure",
				TopHeadlines:    []openbbprovider.NewsHeadline{{Title: "Treasury firms expand Solana exposure", Source: "Decrypt", Sentiment: "bullish"}},
			},
			"BTCUSDT": {
				LookupSymbol:    "BTC-USD",
				Technical:       &openbbprovider.TechnicalSummary{Close: 85000, Return1D: 1.8, Return5D: 6.2, Return20D: 11.0, RSI14: 64.8, MACD: 144.2, MACDSignal: 120.1, MACDHist: 24.1, SMA20: 83000, SMA50: 79000, EMA20: 83600, Volatility20D: 42.0, Trend: "bullish", Momentum: "positive"},
				Sentiment:       &openbbprovider.SentimentSummary{Label: "bullish", Score: 0.28, BullishCount: 3, BearishCount: 1, NeutralCount: 0},
				HeadlineSummary: "ETF inflows remain firm",
			},
		},
	}

	prompt := engine.BuildUserPrompt(ctx)

	checks := []string{
		"## OpenBB Macro & Cross-Asset Context",
		"SPY risk-on, DXY cooling, BTC benchmark firm",
		"OpenBB Enrichment (PIPPIN-USD)",
		"News sentiment: bullish",
		"Treasury firms expand Solana exposure",
		"OpenBB Enrichment (BTC-USD)",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Fatalf("expected prompt to contain %q\nPrompt:\n%s", check, prompt)
		}
	}
}
