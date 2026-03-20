#!/usr/bin/env python3
import json
import math
import statistics
import sys
from datetime import date, datetime, timedelta

from openbb import obb

POSITIVE_WORDS = {
    "surge", "rally", "breakout", "beat", "growth", "bull", "bullish", "soar", "gain",
    "adoption", "approval", "record", "optimism", "strong", "rebound", "upside", "expands",
    "buyback", "outperform", "inflow", "upgrade", "launch", "partnership", "institutional"
}
NEGATIVE_WORDS = {
    "drop", "plunge", "fall", "bear", "bearish", "selloff", "lawsuit", "hack", "liquidation",
    "downgrade", "weak", "risk", "uncertain", "outflow", "crash", "decline", "fraud", "ban",
    "investigation", "delay", "loss", "cuts", "pressure", "volatile", "fear"
}


def to_iso(value):
    if value is None:
        return ""
    if isinstance(value, datetime):
        return value.isoformat()
    if isinstance(value, date):
        return value.isoformat()
    return str(value)


def safe_float(value):
    try:
        if value is None:
            return None
        number = float(value)
        if math.isnan(number) or math.isinf(number):
            return None
        return number
    except Exception:
        return None


def mean(values):
    values = [v for v in values if v is not None]
    return sum(values) / len(values) if values else None


def normalize_symbol(symbol: str) -> str:
    symbol = (symbol or "").strip().upper()
    if not symbol:
        return symbol
    if "-" in symbol or "=" in symbol or symbol.startswith("^"):
        return symbol
    for quote in ("USDT", "USDC", "BUSD", "USD"):
        if symbol.endswith(quote) and len(symbol) > len(quote):
            base = symbol[: -len(quote)]
            return f"{base}-USD"
    return symbol


def score_text(text: str) -> float:
    if not text:
        return 0.0
    tokens = [tok.strip(".,:;!?()[]{}\"'`").lower() for tok in text.split()]
    if not tokens:
        return 0.0
    score = 0
    for token in tokens:
        if token in POSITIVE_WORDS:
            score += 1
        elif token in NEGATIVE_WORDS:
            score -= 1
    return max(-1.0, min(1.0, score / max(3, len(tokens) // 3)))


def sentiment_label(score: float) -> str:
    if score >= 0.15:
        return "bullish"
    if score <= -0.15:
        return "bearish"
    return "neutral"


def summarize_sentiment(items):
    bullish = bearish = neutral = 0
    total = 0.0
    for item in items:
        score = safe_float(item.get("score")) or 0.0
        total += score
        label = item.get("sentiment") or sentiment_label(score)
        if label == "bullish":
            bullish += 1
        elif label == "bearish":
            bearish += 1
        else:
            neutral += 1
    avg = total / len(items) if items else 0.0
    return {
        "label": sentiment_label(avg),
        "score": round(avg, 3),
        "bullish_count": bullish,
        "bearish_count": bearish,
        "neutral_count": neutral,
    }


def headline_summary(items):
    if not items:
        return "No recent OpenBB news available."
    titles = [item["title"] for item in items[:3] if item.get("title")]
    if not titles:
        return "OpenBB returned recent articles without concise headlines."
    return " | ".join(titles)


def get_model_dict(obj):
    if hasattr(obj, "model_dump"):
        return obj.model_dump()
    if isinstance(obj, dict):
        return obj
    return vars(obj)


def fetch_news(symbol: str, limit: int):
    if limit <= 0:
        return []
    try:
        result = obb.news.company(symbol=symbol, limit=limit)
        articles = []
        for item in (result.results or [])[:limit]:
            record = get_model_dict(item)
            text = " ".join(
                part for part in [record.get("title"), record.get("summary"), record.get("text"), record.get("excerpt")] if part
            )
            score = score_text(text)
            articles.append(
                {
                    "date": to_iso(record.get("date")),
                    "title": record.get("title") or "",
                    "source": record.get("source") or "",
                    "summary": record.get("summary") or record.get("text") or record.get("excerpt") or "",
                    "url": record.get("url") or "",
                    "sentiment": sentiment_label(score),
                    "score": round(score, 3),
                }
            )
        return articles
    except Exception:
        return []


def ema_series(values, period: int):
    if len(values) < period:
        return []
    multiplier = 2 / (period + 1)
    seed = sum(values[:period]) / period
    output = [None] * (period - 1) + [seed]
    ema_val = seed
    for value in values[period:]:
        ema_val = ((value - ema_val) * multiplier) + ema_val
        output.append(ema_val)
    return output


def rsi(values, period: int = 14):
    if len(values) <= period:
        return None
    gains = []
    losses = []
    for idx in range(1, period + 1):
        delta = values[idx] - values[idx - 1]
        gains.append(max(delta, 0))
        losses.append(max(-delta, 0))
    avg_gain = sum(gains) / period
    avg_loss = sum(losses) / period

    for idx in range(period + 1, len(values)):
        delta = values[idx] - values[idx - 1]
        gain = max(delta, 0)
        loss = max(-delta, 0)
        avg_gain = ((avg_gain * (period - 1)) + gain) / period
        avg_loss = ((avg_loss * (period - 1)) + loss) / period

    if avg_loss == 0:
        return 100.0 if avg_gain > 0 else 50.0
    rs = avg_gain / avg_loss
    return 100 - (100 / (1 + rs))


def compute_technical(symbol: str, history_days: int):
    start_date = (datetime.utcnow().date() - timedelta(days=max(history_days * 2, history_days + 30))).isoformat()
    result = obb.equity.price.historical(symbol, provider="yfinance", start_date=start_date)
    rows = [get_model_dict(item) for item in (result.results or [])]
    closes = [safe_float(row.get("close")) for row in rows]
    closes = [value for value in closes if value is not None]
    if len(closes) < 30:
        raise ValueError(f"insufficient price history for {symbol}")

    latest = closes[-1]

    def pct_return(window):
        if len(closes) <= window:
            return None
        anchor = closes[-window - 1]
        if not anchor:
            return None
        return ((latest / anchor) - 1) * 100

    sma20 = mean(closes[-20:])
    sma50 = mean(closes[-50:])
    ema20_series = ema_series(closes, 20)
    ema12_series = ema_series(closes, 12)
    ema26_series = ema_series(closes, 26)
    ema20 = ema20_series[-1] if ema20_series else None
    macd_values = []
    for short_val, long_val in zip(ema12_series, ema26_series):
        if short_val is not None and long_val is not None:
            macd_values.append(short_val - long_val)
    macd = macd_values[-1] if macd_values else None
    signal_values = ema_series(macd_values, 9)
    signal = signal_values[-1] if signal_values else None
    macd_hist = (macd - signal) if macd is not None and signal is not None else None
    rsi14 = rsi(closes, 14)

    daily_returns = []
    for prev, curr in zip(closes[:-1], closes[1:]):
        if prev:
            daily_returns.append((curr / prev) - 1)
    vol20 = None
    if len(daily_returns) >= 20:
        vol20 = statistics.pstdev(daily_returns[-20:]) * math.sqrt(365) * 100

    trend = "neutral"
    if sma20 is not None and sma50 is not None:
        if latest > sma20 > sma50:
            trend = "bullish"
        elif latest < sma20 < sma50:
            trend = "bearish"
    momentum = "neutral"
    if rsi14 is not None:
        if rsi14 >= 65:
            momentum = "overbought"
        elif rsi14 <= 35:
            momentum = "oversold"
        elif rsi14 >= 55:
            momentum = "positive"
        elif rsi14 <= 45:
            momentum = "negative"

    return {
        "close": round(latest, 6),
        "return_1d": round(pct_return(1) or 0.0, 3),
        "return_5d": round(pct_return(5) or 0.0, 3),
        "return_20d": round(pct_return(20) or 0.0, 3),
        "sma20": round(sma20, 6) if sma20 is not None else None,
        "sma50": round(sma50, 6) if sma50 is not None else None,
        "ema20": round(ema20, 6) if ema20 is not None else None,
        "rsi14": round(rsi14, 3) if rsi14 is not None else None,
        "macd": round(macd, 6) if macd is not None else None,
        "macd_signal": round(signal, 6) if signal is not None else None,
        "macd_hist": round(macd_hist, 6) if macd_hist is not None else None,
        "volatility_20d": round(vol20, 3) if vol20 is not None else None,
        "trend": trend,
        "momentum": momentum,
    }


def enrich_symbol(raw_symbol: str, history_days: int, news_limit: int, include_news: bool, include_technical: bool):
    lookup_symbol = normalize_symbol(raw_symbol)
    output = {"input_symbol": raw_symbol, "lookup_symbol": lookup_symbol}
    if include_technical:
        output["technical"] = compute_technical(lookup_symbol, history_days)
    if include_news:
        headlines = fetch_news(lookup_symbol, news_limit)
        output["top_headlines"] = headlines[:news_limit]
        output["sentiment"] = summarize_sentiment(headlines)
        output["headline_summary"] = headline_summary(headlines)
    return output


def enrich_macro(symbols, news_limit: int):
    benchmarks = []
    for symbol in symbols:
        lookup_symbol = normalize_symbol(symbol)
        try:
            item = enrich_symbol(lookup_symbol, 90, max(2, min(news_limit, 3)), True, True)
            technical = item.get("technical") or {}
            benchmarks.append(
                {
                    "symbol": symbol,
                    "lookup_symbol": lookup_symbol,
                    "last_price": technical.get("close"),
                    "return_1d": technical.get("return_1d"),
                    "return_5d": technical.get("return_5d"),
                    "return_20d": technical.get("return_20d"),
                    "trend": technical.get("trend"),
                    "sentiment": item.get("sentiment"),
                    "top_headlines": item.get("top_headlines", [])[:2],
                    "headline_summary": item.get("headline_summary") or "",
                }
            )
        except Exception:
            continue

    rate_context = None
    try:
        rates = obb.economy.interest_rates(provider="oecd", country="united_states")
        rows = [get_model_dict(item) for item in (rates.results or [])]
        values = [row for row in rows if safe_float(row.get("value")) is not None]
        if len(values) >= 2:
            latest = values[-1]
            previous = values[-2]
            latest_value = safe_float(latest.get("value")) or 0.0
            previous_value = safe_float(previous.get("value")) or 0.0
            rate_context = {
                "country": latest.get("country") or "united_states",
                "latest": round(latest_value * 100, 3),
                "previous": round(previous_value * 100, 3),
                "delta": round((latest_value - previous_value) * 100, 3),
                "as_of": to_iso(latest.get("date")),
            }
    except Exception:
        rate_context = None

    summary_parts = []
    for benchmark in benchmarks[:4]:
        r1 = benchmark.get("return_1d")
        r5 = benchmark.get("return_5d")
        if r1 is None or r5 is None:
            continue
        summary_parts.append(
            f"{benchmark['symbol']}: 1D {r1:+.2f}%, 5D {r5:+.2f}%, trend={benchmark.get('trend', 'neutral')}"
        )
    if rate_context:
        summary_parts.append(
            f"US policy rate proxy {rate_context['latest']:.2f}% (delta {rate_context['delta']:+.2f}pp, as of {rate_context['as_of']})"
        )

    return {
        "summary": " | ".join(summary_parts),
        "benchmarks": benchmarks,
        "interest_rates": rate_context,
    }


def main():
    request = json.load(sys.stdin)
    symbols = request.get("symbols") or []
    macro_symbols = request.get("macro_symbols") or []
    history_days = int(request.get("history_days") or 90)
    news_limit = int(request.get("news_limit") or 4)
    include_macro = bool(request.get("include_macro"))
    include_news = bool(request.get("include_news"))
    include_technical = bool(request.get("include_technical"))

    response = {
        "generated_at": datetime.utcnow().isoformat() + "Z",
        "symbols": {},
        "errors": [],
    }

    if include_macro and macro_symbols:
        try:
            response["macro"] = enrich_macro(macro_symbols, news_limit)
        except Exception as exc:
            response["errors"].append(f"macro: {exc}")

    for symbol in symbols:
        try:
            response["symbols"][symbol] = enrich_symbol(symbol, history_days, news_limit, include_news, include_technical)
        except Exception as exc:
            response["errors"].append(f"{symbol}: {exc}")

    json.dump(response, sys.stdout, default=to_iso)


if __name__ == "__main__":
    main()
