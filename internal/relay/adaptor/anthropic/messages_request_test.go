package anthropic

import "testing"

func TestValidateMessagesRequest(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name: "valid",
			raw: `{
				"model":"claude-sonnet-4-6",
				"messages":[{"role":"user","content":"hello"}],
				"max_tokens":128
			}`,
		},
		{
			name: "missing model",
			raw: `{
				"messages":[{"role":"user","content":"hello"}]
			}`,
			wantErr: "field model is required",
		},
		{
			name: "missing messages",
			raw: `{
				"model":"claude-sonnet-4-6"
			}`,
			wantErr: "field messages is required",
		},
		{
			name: "invalid max tokens",
			raw: `{
				"model":"claude-sonnet-4-6",
				"messages":[{"role":"user","content":"hello"}],
				"max_tokens":2147483647
			}`,
			wantErr: "max_tokens is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessagesRequest([]byte(tt.raw))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateMessagesRequest returned error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateMessagesRequest error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseMessagesRequestMeta(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"hello"}],
		"max_tokens":256,
		"stream":true,
		"temperature":0.2
	}`)

	requestMeta, err := ParseMessagesRequestMeta(raw)
	if err != nil {
		t.Fatalf("ParseMessagesRequestMeta returned error: %v", err)
	}
	if requestMeta.Model != "claude-sonnet-4-6" {
		t.Fatalf("requestMeta.Model = %q, want %q", requestMeta.Model, "claude-sonnet-4-6")
	}
	if requestMeta.MaxTokens != 256 {
		t.Fatalf("requestMeta.MaxTokens = %d, want %d", requestMeta.MaxTokens, 256)
	}
	if !requestMeta.Stream {
		t.Fatalf("requestMeta.Stream = false, want true")
	}
}
