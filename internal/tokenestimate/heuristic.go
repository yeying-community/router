package tokenestimate

import (
	"math"
	"strings"
	"unicode"
)

type providerFamily string

const (
	familyOpenAI    providerFamily = "openai"
	familyAnthropic providerFamily = "anthropic"
	familyGemini    providerFamily = "gemini"
	familyUnknown   providerFamily = "unknown"
)

type multipliers struct {
	Word       float64
	Number     float64
	CJK        float64
	Symbol     float64
	MathSymbol float64
	URLDelim   float64
	AtSign     float64
	Emoji      float64
	Newline    float64
	Space      float64
}

var familyMultipliers = map[providerFamily]multipliers{
	familyGemini: {
		Word: 1.15, Number: 2.8, CJK: 0.68, Symbol: 0.38, MathSymbol: 1.05, URLDelim: 1.2, AtSign: 2.5, Emoji: 1.08, Newline: 1.15, Space: 0.2,
	},
	familyAnthropic: {
		Word: 1.13, Number: 1.63, CJK: 1.21, Symbol: 0.4, MathSymbol: 4.52, URLDelim: 1.26, AtSign: 2.82, Emoji: 2.6, Newline: 0.89, Space: 0.39,
	},
	familyOpenAI: {
		Word: 1.02, Number: 1.55, CJK: 0.85, Symbol: 0.4, MathSymbol: 2.68, URLDelim: 1.0, AtSign: 2.0, Emoji: 2.12, Newline: 0.5, Space: 0.42,
	},
	familyUnknown: {
		Word: 1.02, Number: 1.55, CJK: 0.85, Symbol: 0.4, MathSymbol: 2.68, URLDelim: 1.0, AtSign: 2.0, Emoji: 2.12, Newline: 0.5, Space: 0.42,
	},
}

func detectFamily(model string) providerFamily {
	lower := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(lower, "claude"):
		return familyAnthropic
	case strings.Contains(lower, "gemini"):
		return familyGemini
	case strings.HasPrefix(lower, "gpt-"), isOReasoningFamily(lower), strings.HasPrefix(lower, "chatgpt"):
		return familyOpenAI
	default:
		return familyUnknown
	}
}

func isOReasoningFamily(model string) bool {
	normalized := strings.TrimSpace(strings.ToLower(model))
	if len(normalized) < 2 || normalized[0] != 'o' || !unicode.IsDigit(rune(normalized[1])) {
		return false
	}
	if len(normalized) == 2 {
		return true
	}
	next := normalized[2]
	return next == '-' || next == '_' || next == '.'
}

func estimateHeuristicText(text string, family providerFamily) int {
	m := familyMultipliers[family]
	type wordType int
	const (
		none wordType = iota
		latin
		number
	)
	current := none
	var count float64
	for _, r := range text {
		if unicode.IsSpace(r) {
			current = none
			if r == '\n' || r == '\t' {
				count += m.Newline
			} else {
				count += m.Space
			}
			continue
		}
		if isCJK(r) {
			current = none
			count += m.CJK
			continue
		}
		if isEmoji(r) {
			current = none
			count += m.Emoji
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			next := latin
			if unicode.IsNumber(r) {
				next = number
			}
			if current == none || current != next {
				if next == number {
					count += m.Number
				} else {
					count += m.Word
				}
				current = next
			}
			continue
		}
		current = none
		switch {
		case isMathSymbol(r):
			count += m.MathSymbol
		case r == '@':
			count += m.AtSign
		case isURLDelim(r):
			count += m.URLDelim
		default:
			count += m.Symbol
		}
	}
	return int(math.Ceil(count))
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || (r >= 0x3040 && r <= 0x30FF) || (r >= 0xAC00 && r <= 0xD7A3)
}

func isEmoji(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF)
}

func isMathSymbol(r rune) bool {
	if r >= 0x2200 && r <= 0x22FF {
		return true
	}
	if r >= 0x2A00 && r <= 0x2AFF {
		return true
	}
	if r >= 0x1D400 && r <= 0x1D7FF {
		return true
	}
	return strings.ContainsRune("∑∫∂√∞≤≥≠≈±×÷", r)
}

func isURLDelim(r rune) bool {
	return strings.ContainsRune("/:?&=;#%", r)
}
