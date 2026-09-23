package ratio

import (
	"encoding/json"
	"testing"
)

// The expected values below are taken from the current upstream default table
// (Calcium-Ion/new-api setting/ratio_setting/model_ratio.go, fetched 2026-09-23).
// Each case quotes the reference line it was read from.

func TestGetModelRatioFallback(t *testing.T) {
	// No table anywhere has these names: they must be billed at 1x, not at the old 30x.
	unknown := []string{
		"deepseek-flash", // reseller name served by the user's own instance
		"totally-made-up-model",
		"Doubao-pro-32k", // doubao has no entry in either table
	}
	for _, model := range unknown {
		if got := GetModelRatio(model, 1); got != 1 {
			t.Errorf("GetModelRatio(%q, 1) = %v, want 1", model, got)
		}
	}
	if got := GetCompletionRatio("totally-made-up-model", 1); got != 1 {
		t.Errorf("GetCompletionRatio(\"totally-made-up-model\", 1) = %v, want 1", got)
	}
}

func TestModelRatioSpotChecks(t *testing.T) {
	cases := []struct {
		model string
		want  float64
		ref   string
	}{
		{"gpt-4o", 1.25, `"gpt-4o": 1.25, // $2.5 / 1M tokens`},
		{"gpt-4o-mini", 0.075, `"gpt-4o-mini": 0.075,`},
		{"gpt-3.5-turbo", 0.25, `"gpt-3.5-turbo": 0.25,`},
		{"gpt-4.1", 1.0, `"gpt-4.1": 1.0, // $2 / 1M tokens`},
		{"o3", 1.0, `"o3": 1.0, // $2 / 1M tokens`},
		{"o4-mini", 0.55, `"o4-mini": 0.55, // $1.1 / 1M tokens`},
		{"claude-3-5-sonnet-20241022", 1.5, `"claude-3-5-sonnet-20241022": 1.5,`},
		{"claude-sonnet-4-20250514", 1.5, `"claude-sonnet-4-20250514": 1.5,`},
		{"gemini-2.5-pro", 0.625, `"gemini-2.5-pro": 0.625,`},
		{"text-embedding-v1", 0.05, `"text-embedding-v1": 0.05, // ￥0.0007 / 1k tokens`},
		{"deepseek-chat", 0.27 / 2, `"deepseek-chat": 0.27 / 2,`},
		{"deepseek-reasoner", 0.55 / 2, `"deepseek-reasoner": 0.55 / 2, // 0.55 / 1k tokens`},
	}
	for _, c := range cases {
		if got := GetModelRatio(c.model, 1); got != c.want {
			t.Errorf("GetModelRatio(%q, 1) = %v, want %v (reference: %s)", c.model, got, c.want, c.ref)
		}
	}
}

func TestCompletionRatioSpotChecks(t *testing.T) {
	cases := []struct {
		model string
		want  float64
		ref   string
	}{
		// upstream getHardcodedCompletionModelRatio
		{"gpt-4o", 4, `HasPrefix "gpt-4o" -> return 4`},
		{"gpt-4o-2024-05-13", 3, `name == "gpt-4o-2024-05-13" -> return 3`},
		{"gpt-5", 8, `HasPrefix "gpt-5" && !Contains(name, ".") -> return 8`},
		{"o3-mini", 4, `HasPrefix "o1" || HasPrefix "o3" -> return 4`},
		{"claude-3-5-haiku-20241022", 5, `Contains(name, "claude-3") -> return 5`},
		{"claude-sonnet-4-20250514", 5, `Contains(name, "claude-sonnet-4") -> return 5`},
		{"gemini-2.5-pro", 8, `HasPrefix "gemini-2.5-pro" -> return 8`},
		{"gemini-2.0-flash", 4, `HasPrefix "gemini-2.0" -> return 4`},
		{"gemini-2.5-flash", 2.5 / 0.3, `HasPrefix "gemini-2.5-flash" -> return 2.5 / 0.3`},
		// kept from this repo's own CompletionRatio map / prefixes
		{"deepseek-chat", 0.28 / 0.14, `CompletionRatio["deepseek-chat"] = 0.28 / 0.14`},
		{"deepseek-reasoner", 2.19 / 0.55, `CompletionRatio["deepseek-reasoner"] = 2.19 / 0.55`},
		{"whisper-1", 0, `CompletionRatio["whisper-1"] = 0`},
	}
	for _, c := range cases {
		if got := GetCompletionRatio(c.model, 1); got != c.want {
			t.Errorf("GetCompletionRatio(%q, 1) = %v, want %v (reference: %s)", c.model, got, c.want, c.ref)
		}
	}
}

// Deliberate divergence: the reference still carries Aliyun's 2023 DashScope prices,
// this repo carries the current ones (￥0.0003 / 1k for qwen-turbo, ￥0.0008 / 1k for
// qwen-plus). Adopting the reference numbers would raise these by 40x / 175x.
func TestAliyunQwenKeepsCurrentPrices(t *testing.T) {
	if got, want := GetModelRatio("qwen-turbo", 1), 0.0003*RMB; got != want {
		t.Errorf("GetModelRatio(\"qwen-turbo\", 1) = %v, want %v", got, want)
	}
	if got, want := GetModelRatio("qwen-plus", 1), 0.0008*RMB; got != want {
		t.Errorf("GetModelRatio(\"qwen-plus\", 1) = %v, want %v", got, want)
	}
}

func TestModelRatioJSONRoundTrip(t *testing.T) {
	jsonStr := ModelRatio2JSONString()
	before := GetModelRatio("gpt-4o", 1)
	if err := UpdateModelRatioByJSONString(jsonStr); err != nil {
		t.Fatalf("UpdateModelRatioByJSONString: %v", err)
	}
	if got := GetModelRatio("gpt-4o", 1); got != before {
		t.Errorf("after round trip GetModelRatio(\"gpt-4o\", 1) = %v, want %v", got, before)
	}
	if got := GetModelRatio("totally-made-up-model", 1); got != 1 {
		t.Errorf("after round trip GetModelRatio(\"totally-made-up-model\", 1) = %v, want 1", got)
	}

	completionJSON := CompletionRatio2JSONString()
	completionBefore := GetCompletionRatio("deepseek-chat", 1)
	if err := UpdateCompletionRatioByJSONString(completionJSON); err != nil {
		t.Fatalf("UpdateCompletionRatioByJSONString: %v", err)
	}
	if got := GetCompletionRatio("deepseek-chat", 1); got != completionBefore {
		t.Errorf("after round trip GetCompletionRatio(\"deepseek-chat\", 1) = %v, want %v", got, completionBefore)
	}
}

func TestAddNewMissingRatioBackfillsDefaults(t *testing.T) {
	merged := AddNewMissingRatio("{}")
	var got map[string]float64
	if err := json.Unmarshal([]byte(merged), &got); err != nil {
		t.Fatalf("unmarshal merged ratio: %v", err)
	}
	if len(got) != len(DefaultModelRatio) {
		t.Errorf("merged ratio has %d entries, want %d (all defaults)", len(got), len(DefaultModelRatio))
	}
	for model, want := range map[string]float64{
		"gpt-4o":        1.25,
		"gpt-4.1":       1.0,
		"gpt-5":         0.625,
		"deepseek-chat": 0.27 / 2,
	} {
		if got[model] != want {
			t.Errorf("backfilled %q = %v, want %v", model, got[model], want)
		}
	}
	if _, ok := got["deepseek-flash"]; ok {
		t.Error("unknown reseller name deepseek-flash must not be a default entry")
	}
	// an existing key is never overwritten
	kept := AddNewMissingRatio(`{"gpt-4o": 42}`)
	var keptMap map[string]float64
	if err := json.Unmarshal([]byte(kept), &keptMap); err != nil {
		t.Fatalf("unmarshal kept ratio: %v", err)
	}
	if keptMap["gpt-4o"] != 42 {
		t.Errorf("AddNewMissingRatio overwrote an existing key: gpt-4o = %v, want 42", keptMap["gpt-4o"])
	}
}
