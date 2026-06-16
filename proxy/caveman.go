package proxy

import "strings"

// caveman mode levels (based on juliusbrussee/caveman skill spec)
const (
	CavemanModeLite   = "lite"
	CavemanModeFull   = "full"
	CavemanModeUltra  = "ultra"
	CavemanModeWenyan = "wenyan"
)

// cavemanLitePrompt drops filler phrases but keeps normal grammar.
const cavemanLitePrompt = `RESPONSE STYLE: Drop all filler phrases ("I'd be happy to", "Sure!", "Great question!", "Of course!", "Certainly!", "Let me explain", "I hope this helps", etc.). Start your answer directly. Keep normal sentences but cut preamble and sign-offs.`

// cavemanFullPrompt — default mode. Caveman speech: drop articles, conjunctions, pleasantries.
// Keep all technical content. ~75% fewer output tokens.
const cavemanFullPrompt = `RESPONSE STYLE: Talk like caveman. Rules:
- Drop: articles (a, an, the), filler conjunctions (I would, let me, please note), pleasantries
- Keep: all technical content, code, file paths, error messages, numbers, logic
- Use short sentences. Subject + verb + object. No preamble. No sign-off.
- Code blocks: always use. Comments in code: only when non-obvious.
- Lists > paragraphs when ≥3 items.
- Never start with "I", "Sure", "Great", "Of course", "Certainly", "Absolutely".
Example: instead of "The reason your component re-renders is because..." → "New object ref each render. Wrap in useMemo."`

// cavemanUltraPrompt — telegraphic maximum compression.
const cavemanUltraPrompt = `RESPONSE STYLE: Maximum compression. Telegraphic only.
- Zero articles. Zero filler. Zero preamble.
- Noun phrases + arrows/colons for causality.
- Code: always fenced. No prose wrap.
- Errors: cause → fix, no explanation unless asked.
- If answer fits 1 line, 1 line. Never pad.`

// cavemanWenyanPrompt — classical Chinese style for extreme compression.
const cavemanWenyanPrompt = `RESPONSE STYLE: 文言 (wenyan) mode. Classical Chinese compression applied to modern tech.
- Omit subjects when obvious. Verb-first when possible.
- Zero pleasantries. Zero hedging.
- Technical terms stay in English/code. Logic expressed in minimal words.
- Target: ≤30% of normal response length.
- Code blocks: always use, no prose wrap.`

// buildCavemanPrompt returns the caveman instructions for the given level.
// Returns empty string for unknown/off levels.
func buildCavemanPrompt(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case CavemanModeLite:
		return cavemanLitePrompt
	case CavemanModeFull, "":
		return cavemanFullPrompt
	case CavemanModeUltra:
		return cavemanUltraPrompt
	case CavemanModeWenyan:
		return cavemanWenyanPrompt
	}
	return ""
}

// isValidCavemanMode reports whether mode is a known caveman level.
func isValidCavemanMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case CavemanModeLite, CavemanModeFull, CavemanModeUltra, CavemanModeWenyan:
		return true
	}
	return false
}

// injectCavemanInstructions prepends caveman instructions to the system prompt.
// If the prompt is empty, returns only the caveman instructions.
// If the prompt already contains caveman marker, does not double-inject.
func injectCavemanInstructions(prompt, mode string) string {
	instructions := buildCavemanPrompt(mode)
	if instructions == "" {
		return prompt
	}
	// Guard against double injection (e.g. if caveman prompt was already in system prompt).
	if strings.Contains(prompt, "RESPONSE STYLE:") || strings.Contains(prompt, "Talk like caveman") {
		return prompt
	}
	return instructions + "\n\n" + prompt
}

// estimateCavemanTokensSaved estimates the number of tokens saved based on the output tokens and caveman mode.
func estimateCavemanTokensSaved(outputTokens int, mode string) int {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case CavemanModeLite:
		return int(float64(outputTokens) * 0.25)
	case CavemanModeFull, "":
		return outputTokens * 3
	case CavemanModeWenyan:
		return outputTokens * 3
	case CavemanModeUltra:
		return outputTokens * 6
	}
	return 0
}
