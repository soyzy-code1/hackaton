package search

import (
	"strings"
)

// stopWords are the generic words that must not be used to judge whether a
// search result is about the requested subscription.
var stopWords = map[string]bool{
	"подписка": true, "подписку": true, "подписки": true, "подписок": true,
	"цена": true, "цены": true, "цену": true, "стоимость": true,
	"месяц": true, "месяца": true, "месяцев": true, "мес": true,
	"год": true, "года": true, "лет": true,
	"тариф": true, "тарифы": true, "тарифах": true,
	"руб": true, "рублей": true, "рубля": true, "рода": true,
	"в": true, "и": true, "на": true, "с": true, "для": true, "по": true,
	"сколько": true, "стоит": true,
	"аналог": true, "альтернатива": true, "дешевле": true, "дешёвый": true,
}

// SubjectTokens splits a subscription title into the meaningful words used for
// relevance checks. The title "Яндекс Плюс" yields ["яндекс", "плюс"].
func SubjectTokens(title string) []string {
	normalized := Normalize(title)
	tokens := []string{}
	for _, word := range strings.Fields(normalized) {
		if stopWords[word] || len([]rune(word)) < 3 {
			continue
		}
		tokens = append(tokens, word)
	}
	return tokens
}

// IsRelevant reports whether a search hit is about the given subject. A hit is
// relevant when its title or its URL mentions one of the subject tokens.
func IsRelevant(hit Hit, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	haystack := Normalize(hit.Title + " " + hit.URL)
	for _, token := range tokens {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

// FilterRelevant keeps only the hits that are about the subject.
func FilterRelevant(hits []Hit, subject string) []Hit {
	tokens := SubjectTokens(subject)
	if len(tokens) == 0 {
		return hits
	}
	out := make([]Hit, 0, len(hits))
	for _, hit := range hits {
		if IsRelevant(hit, tokens) {
			out = append(out, hit)
		}
	}
	return out
}

// priceKeywords are the words that indicate that a price is a subscription
// price and not an unrelated number from the page.
var priceKeywords = []string{
	"цена", "цены", "стоимость", "подписк", "тариф", "месяц", "мес",
	"год", "годов", "оплат", "абонемент", "в месяц", "за месяц",
}

// ExtractRelevantPrices returns only the prices whose context talks about a
// price, a tariff or a subscription period. Numbers that belong to the rest
// of the page (phone numbers, article years, addresses) are dropped.
func ExtractRelevantPrices(text string) []Price {
	all := ExtractPrices(text)
	if len(all) == 0 {
		return nil
	}
	out := []Price{}
	for _, price := range all {
		if hasPriceKeyword(price.Context) {
			out = append(out, price)
		}
	}
	if len(out) == 0 {
		// Nothing matched the keywords: keep the first prices as a fallback so
		// that the model still gets something to work with.
		if len(all) > 3 {
			all = all[:3]
		}
		return all
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func hasPriceKeyword(context string) bool {
	lowered := strings.ToLower(context)
	for _, keyword := range priceKeywords {
		if strings.Contains(lowered, keyword) {
			return true
		}
	}
	return false
}
