package search

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

// priceRe matches a monetary amount together with its currency marker.
//
// The pattern was validated against real subscription pricing pages:
//
//	99 рублей, 169 рублей, 309 ₽, 1 299 ₽, 9.99 USD, 4,99 €, $12
//
// The Russian inflections are matched with an explicit letter class instead of
// a "\b" word boundary, because the boundary does not work with Cyrillic.
var priceRe = regexp.MustCompile(`(?i)(\d[\d\s.,]{0,9})\s*(₽|руб[а-яё]{0,4}|RUB|USD|EUR|GBP|\$|€)`)

// Price is a single monetary value found on a web page.
type Price struct {
	// Amount is the parsed numeric value.
	Amount decimal.Decimal
	// Currency is the normalised currency code (RUB, USD, EUR, ...).
	Currency string
	// Raw is the exact text that was matched, e.g. "309 ₽".
	Raw string
	// Context is a readable excerpt around the match.
	Context string
}

// Evidence converts the price into the JSON shape handed to the AI model.
func (p Price) Evidence() PriceEvidence {
	return PriceEvidence{
		Amount:   p.Amount,
		Currency: p.Currency,
		Raw:      p.Raw,
		Context:  p.Context,
	}
}

// PriceEvidence is the serialisable form of a price with its context.
type PriceEvidence struct {
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
	Raw      string          `json:"raw"`
	Context  string          `json:"context"`
}

// currencyMap maps the currency markers found in page text to ISO codes.
var currencyMap = map[string]string{
	"₽":        "RUB",
	"usd":      "USD",
	"$":        "USD",
	"eur":      "EUR",
	"€":        "EUR",
	"gbp":      "GBP",
	"£":        "GBP",
	"ruble":    "RUB",
	"rubles":   "RUB",
	"доллар":   "USD",
	"доллара":  "USD",
	"долларов": "USD",
	"евро":     "EUR",
}

// currencyCode maps a matched currency marker to an ISO currency code.
func currencyCode(marker string) (string, bool) {
	lowered := strings.ToLower(strings.TrimSpace(marker))
	if lowered == "" {
		return "", false
	}
	if code, ok := currencyMap[lowered]; ok {
		return code, true
	}
	// Russian inflections: "руб", "руб.", "рублей", "рубля", "рублях" and so on.
	if strings.HasPrefix(lowered, "руб") {
		return "RUB", true
	}
	return "", false
}

// ExtractPrices finds all monetary values in the given plain text and keeps
// the most useful ones (unique amount + currency pairs, sane magnitudes).
func ExtractPrices(text string) []Price {
	if text == "" {
		return nil
	}
	matches := priceRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := []Price{}
	seen := map[string]bool{}
	for _, m := range matches {
		raw := text[m[0]:m[1]]
		number := strings.TrimSpace(text[m[2]:m[3]])
		marker := strings.TrimSpace(text[m[4]:m[5]])
		currency, ok := currencyCode(marker)
		if !ok {
			continue
		}
		amount, err := parseAmount(number)
		if err != nil {
			continue
		}
		if !plausibleAmount(amount) {
			continue
		}
		key := currency + "|" + amount.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Price{
			Amount:   amount,
			Currency: currency,
			Raw:      strings.TrimSpace(raw),
			Context:  priceContext(text, m[0], m[1]),
		})
		if len(out) >= 12 {
			break
		}
	}
	return out
}

// parseAmount converts a number written with spaces, dots or commas into a
// decimal value. "1 299" and "1,299" are thousand separators, "9.99" and
// "9,99" are decimal separators.
func parseAmount(value string) (decimal.Decimal, error) {
	cleaned := strings.ReplaceAll(value, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "\u00a0", "")
	if cleaned == "" {
		return decimal.Zero, strconv.ErrSyntax
	}
	lastComma := strings.LastIndex(cleaned, ",")
	lastDot := strings.LastIndex(cleaned, ".")
	switch {
	case lastComma >= 0 && lastDot >= 0:
		if lastComma > lastDot {
			cleaned = strings.ReplaceAll(cleaned, ".", "")
			cleaned = strings.ReplaceAll(cleaned, ",", ".")
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", "")
		}
	case lastComma >= 0:
		parts := strings.Split(cleaned, ",")
		if len(parts) == 2 && len(parts[1]) == 3 {
			cleaned = parts[0] + parts[1]
		} else {
			cleaned = strings.ReplaceAll(cleaned, ",", ".")
		}
	case lastDot >= 0:
		parts := strings.Split(cleaned, ".")
		if len(parts) == 2 && len(parts[1]) == 3 {
			cleaned = parts[0] + parts[1]
		}
	}
	return decimal.NewFromString(cleaned)
}

// plausibleAmount filters out values that cannot be a subscription price.
func plausibleAmount(amount decimal.Decimal) bool {
	if amount.IsNegative() {
		return false
	}
	if amount.LessThan(decimal.NewFromInt(1)) {
		return false
	}
	if amount.GreaterThan(decimal.NewFromInt(1_000_000)) {
		return false
	}
	return true
}

// priceContext returns a readable excerpt around a price match.
func priceContext(text string, start, end int) string {
	const window = 130
	from := start - window
	if from < 0 {
		from = 0
	}
	to := end + window
	if to > len(text) {
		to = len(text)
	}
	fragment := strings.TrimSpace(text[from:to])
	fragment = strings.Join(strings.Fields(fragment), " ")
	return fragment
}

// CurrencyToRUB converts an amount into roubles using a static reference
// table. The rates are intentionally coarse: they are only used to decide
// whether an alternative is cheaper, the exact numbers always come from the
// source page.
var CurrencyToRUB = map[string]decimal.Decimal{
	"RUB": decimal.NewFromInt(1),
	"USD": decimal.NewFromInt(100),
	"EUR": decimal.NewFromInt(110),
	"GBP": decimal.NewFromInt(125),
}

// ToRUB converts a price into roubles, returning false when the currency is
// unknown.
func (p Price) ToRUB() (decimal.Decimal, bool) {
	rate, ok := CurrencyToRUB[p.Currency]
	if !ok {
		return decimal.Zero, false
	}
	return p.Amount.Mul(rate), true
}
