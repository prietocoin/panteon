package cervantes

import "testing"

func TestNewLLMWriterFromDeepSeekAlias(t *testing.T) {
	t.Setenv("PANTEON_LLM_BASE_URL", "")
	t.Setenv("PANTEON_LLM_MODEL", "")
	t.Setenv("PANTEON_LLM_API_KEY", "")
	t.Setenv("PANTEON_LLM_DEEPSEEK", "sk-test")

	writer := NewLLMWriterFromEnv()
	if writer == nil {
		t.Fatal("writer nil")
	}
	if writer.BaseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("base = %q", writer.BaseURL)
	}
	if writer.Model != "deepseek-chat" {
		t.Fatalf("model = %q", writer.Model)
	}
	if writer.APIKey != "sk-test" {
		t.Fatalf("key = %q", writer.APIKey)
	}
}
