# Porting Blueprint: Integrating Caveman Mode (JS to Go)

This document serves as a porting guide and architectural blueprint for migrating **Caveman Mode** (a system-instruction-based output token saver) from **9router** (Node.js/JS) to a Go-based proxy project like **Kiro-Go**.

---

## 1. Core Architectural Concept

**Caveman Mode** intercepts outgoing LLM requests and appends custom formatting instructions to the `system` prompt (or system message) before the request is sent to the provider. 

By forcing the model to adopt a terse, telegraphic response style, it reduces output token usage by **60% to 90%** while preserving technical accuracy, code blocks, URLs, and errors.

```
[Incoming Request] 
      │
      ▼
[Request Parsing & Translation]
      │
      ▼
[Caveman Injector Filter] ◄─── Reads Configuration (Enabled? Level?)
      │
      ▼ (System prompt modified with Terse/Caveman instructions)
[Upstream LLM Provider]
```

---

## 2. Key Differences: Node.js vs. Go Implementations

| Component | Node.js (9router) | Go (Kiro-Go) |
| :--- | :--- | :--- |
| **Config Store** | SQLite database (via setting repo) | Thread-safe JSON config + Mutex protection |
| **Logic Location** | `rtk/caveman.js` called in `chatCore.js` | `proxy/caveman.go` called in `proxy/translator.go` |
| **Format Handling** | Switch-case on format string (Claude, Gemini, etc.) | Translation happens in native struct mapping |
| **Prompt injection** | In-place modification of JSON payload structure | Appended to string variables before structural building |

---

## 3. Step-by-Step Porting Checklist

### Step 1: Configuration Fields
Define configuration keys for toggle state and severity level:
- **`CavemanEnabled` / `CavemanMode`**: String or boolean flag. Values: `off`, `lite`, `full` (default), `ultra`, `wenyan`.
- **`CavemanLevel`**: Configures the compression tier.

**Go Config Definition Example (`config/config.go`):**
```go
type Config struct {
    // ... other settings ...
    
    // CavemanMode injects caveman-style compact response instructions into every request's
    // system prompt, reducing output tokens by ~75% while preserving technical accuracy.
    // Values: "" or "off" (disabled), "lite", "full" (default when enabled), "ultra", "wenyan".
    CavemanMode string `json:"cavemanMode,omitempty"`
}
```

---

### Step 2: Defining the Prompt Levels

Create a file dedicated to the prompts. These prompts are crucial for instructing the LLM on *how* to shorten its output.

*   **Lite**: Drop filler words, preamble, and sign-offs. Keep normal grammar.
*   **Full**: Talk like a caveman (drop articles `a/an/the`, conjunctions). Use fragments. Keep code blocks intact.
*   **Ultra**: Maximum telegraphic style. Use causality arrows `X → Y`. No prose explanation.
*   **Wenyan**: Classical Chinese compression applied to technical instructions.

**Go Prompt Definition (`proxy/caveman.go`):**
```go
package proxy

import "strings"

const (
	CavemanModeLite   = "lite"
	CavemanModeFull   = "full"
	CavemanModeUltra  = "ultra"
	CavemanModeWenyan = "wenyan"
)

const cavemanLitePrompt = `RESPONSE STYLE: Drop all filler phrases ("I'd be happy to", "Sure!", "Great question!", "Of course!", "Certainly!", "Let me explain", "I hope this helps", etc.). Start your answer directly. Keep normal sentences but cut preamble and sign-offs.`

const cavemanFullPrompt = `RESPONSE STYLE: Talk like caveman. Rules:
- Drop: articles (a, an, the), filler conjunctions (I would, let me, please note), pleasantries
- Keep: all technical content, code, file paths, error messages, numbers, logic
- Use short sentences. Subject + verb + object. No preamble. No sign-off.
- Code blocks: always use. Comments in code: only when non-obvious.
- Lists > paragraphs when ≥3 items.
- Never start with "I", "Sure", "Great", "Of course", "Certainly", "Absolutely".
Example: instead of "The reason your component re-renders is because..." -> "New object ref each render. Wrap in useMemo."`

const cavemanUltraPrompt = `RESPONSE STYLE: Maximum compression. Telegraphic only.
- Zero articles. Zero filler. Zero preamble.
- Noun phrases + arrows/colons for causality.
- Code: always fenced. No prose wrap.
- Errors: cause -> fix, no explanation unless asked.
- If answer fits 1 line, 1 line. Never pad.`

const cavemanWenyanPrompt = `RESPONSE STYLE: 文言 (wenyan) mode. Classical Chinese compression applied to modern tech.
- Omit subjects when obvious. Verb-first when possible.
- Zero pleasantries. Zero hedging.
- Technical terms stay in English/code. Logic expressed in minimal words.
- Target: ≤30% of normal response length.
- Code blocks: always use, no prose wrap.`
```

---

### Step 3: Implement Injection Logic

You must prepend the instructions to the system prompt. Make sure to guard against double-injection in case the prompt already has the instructions.

**Go Injection Code (`proxy/caveman.go`):**
```go
// buildCavemanPrompt returns the caveman instructions for the given level.
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

// injectCavemanInstructions prepends caveman instructions to the system prompt.
func injectCavemanInstructions(prompt, mode string) string {
	instructions := buildCavemanPrompt(mode)
	if instructions == "" {
		return prompt
	}
	// Guard against double injection (if caveman prompt was already injected)
	if strings.Contains(prompt, "RESPONSE STYLE:") || strings.Contains(prompt, "Talk like caveman") {
		return prompt
	}
	if prompt == "" {
		return instructions
	}
	return instructions + "\n\n" + prompt
}
```

---

### Step 4: Hooks into the Pipeline

The injector must run **during translation** after the system prompt has been extracted, but before the final structure is generated and dispatched to the upstream API.

**Example Pipeline Hook (`proxy/translator.go`):**
```go
func applyPromptFiltersWithCaveman(prompt, cavemanOverride string) string {
	prompt = strings.TrimSpace(prompt)
	
	// If the prompt is empty, we still want to inject caveman mode instructions
	// so the model follows the constraints even if the client didn't supply a system message.
	if prompt == "" {
		mode := cavemanOverride
		if mode == "" {
			mode = config.GetCavemanMode()
		}
		if mode != "" && mode != "off" {
			return buildCavemanPrompt(mode)
		}
		return ""
	}

	// ... other prompt filtering/cleaning rules ...

	// Inject Caveman Mode
	mode := cavemanOverride
	if mode == "" {
		mode = config.GetCavemanMode()
	}
	if mode != "" && mode != "off" {
		prompt = injectCavemanInstructions(strings.TrimSpace(prompt), mode)
	}

	return strings.TrimSpace(prompt)
}
```

---

## 4. Verification and Validation

After implementation, verify the following:

1. **System Prompt Appending**: Inspect the outbound raw payload logs. Ensure the `system` instruction starts with the corresponding `RESPONSE STYLE:` header.
2. **No Double Injection**: Send multiple messages in the same session. Ensure that history compacting/pruning does not repeatedly stack multiple `RESPONSE STYLE:` prefixes onto older turns.
3. **Empty Prompt Safety**: Trigger a chat request with no developer/system instructions. Confirm that the system prompt receives the Caveman instructions correctly instead of remaining blank.
4. **Token Savings Check**: Compare the `completion_tokens` count with and without Caveman Mode. You should see a reduction of output lengths by up to 75%.
