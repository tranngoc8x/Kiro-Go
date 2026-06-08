package proxy

import (
	"strings"
	"testing"
)

func TestBuildCavemanPrompt(t *testing.T) {
	tests := []struct {
		mode     string
		wantSubs []string // substrings expected in output
		wantEmpty bool
	}{
		{
			mode:     "lite",
			wantSubs: []string{"RESPONSE STYLE", "filler"},
		},
		{
			mode:     "full",
			wantSubs: []string{"RESPONSE STYLE", "caveman", "articles"},
		},
		{
			mode:     "ultra",
			wantSubs: []string{"RESPONSE STYLE", "Maximum compression"},
		},
		{
			mode:     "wenyan",
			wantSubs: []string{"文言", "wenyan"},
		},
		{
			mode:      "off",
			wantEmpty: true,
		},
		{
			mode:      "unknown",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := buildCavemanPrompt(tt.mode)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("buildCavemanPrompt(%q) = %q, want empty", tt.mode, got)
				}
				return
			}
			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("buildCavemanPrompt(%q) missing %q\ngot: %q", tt.mode, sub, got)
				}
			}
		})
	}
}

func TestIsValidCavemanMode(t *testing.T) {
	valid := []string{"lite", "full", "ultra", "wenyan", "FULL", " lite "}
	for _, m := range valid {
		if !isValidCavemanMode(m) {
			t.Errorf("isValidCavemanMode(%q) = false, want true", m)
		}
	}

	invalid := []string{"", "off", "medium", "caveman"}
	for _, m := range invalid {
		if isValidCavemanMode(m) {
			t.Errorf("isValidCavemanMode(%q) = true, want false", m)
		}
	}
}

func TestInjectCavemanInstructions(t *testing.T) {
	t.Run("empty prompt gets caveman only", func(t *testing.T) {
		got := injectCavemanInstructions("", "full")
		if !strings.Contains(got, "RESPONSE STYLE") {
			t.Errorf("expected caveman prompt, got %q", got)
		}
	})

	t.Run("prompt gets caveman prepended", func(t *testing.T) {
		const orig = "You are a helpful assistant."
		got := injectCavemanInstructions(orig, "lite")
		if !strings.HasPrefix(got, "RESPONSE STYLE") {
			t.Errorf("caveman not prepended, got: %q", got)
		}
		if !strings.Contains(got, orig) {
			t.Errorf("original prompt missing from result: %q", got)
		}
	})

	t.Run("off mode returns prompt unchanged", func(t *testing.T) {
		const orig = "You are a helpful assistant."
		got := injectCavemanInstructions(orig, "off")
		if got != orig {
			t.Errorf("expected original prompt unchanged, got %q", got)
		}
	})

	t.Run("no double injection", func(t *testing.T) {
		// Simulate a prompt that already has RESPONSE STYLE (e.g. already injected)
		const alreadyInjected = "RESPONSE STYLE: Talk like caveman.\n\nYou are helpful."
		got := injectCavemanInstructions(alreadyInjected, "full")
		// Should not prepend again
		count := strings.Count(got, "RESPONSE STYLE")
		if count != 1 {
			t.Errorf("double injection detected: RESPONSE STYLE appears %d times", count)
		}
	})

	t.Run("unknown mode returns prompt unchanged", func(t *testing.T) {
		const orig = "System instructions here."
		got := injectCavemanInstructions(orig, "unknown")
		if got != orig {
			t.Errorf("expected prompt unchanged for unknown mode, got %q", got)
		}
	})
}
