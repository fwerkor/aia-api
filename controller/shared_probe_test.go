package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSharedProbeCatalogValidation(t *testing.T) {
	p := model.SharedProvider{Protocol: "chat"}
	candidates := []model.SharedModel{
		{ID: 1, Code: "deepseek-flash", Protocol: "chat"},
		{ID: 2, Code: "kimi-k2.5", Protocol: "chat"},
		{ID: 3, Code: "test-draft", Protocol: "chat"},
	}
	calls := []string{}
	discovered := func(context.Context, model.SharedProvider, string) (map[string]bool, error) {
		return map[string]bool{"deepseek-flash": true, "kimi-k2.5": true}, nil
	}
	verify := func(_ context.Context, p model.SharedProvider, _ string) error {
		calls = append(calls, p.Model)
		if p.Model == "kimi-k2.5" {
			return errors.New("upstream HTTP 404")
		}
		return nil
	}
	succeeded, results := sharedProbeWith(context.Background(), p, "key", candidates, discovered, verify)
	require.Equal(t, []string{"deepseek-flash", "kimi-k2.5"}, calls)
	require.Len(t, succeeded, 1)
	require.Equal(t, "deepseek-flash", succeeded[0].Code)
	require.Len(t, results, 3)
	require.True(t, results[0].Verified)
	require.False(t, results[1].Verified)
	require.Equal(t, "not listed by upstream", results[2].Reason)
}
func TestSharedProbeMissingListingFallsBackToCalls(t *testing.T) {
	p := model.SharedProvider{Protocol: "responses"}
	candidates := []model.SharedModel{{Code: "gpt-6-sol"}, {Code: "gpt-6-luna"}}
	callCount := 0
	succeeded, _ := sharedProbeWith(context.Background(), p, "key", candidates,
		func(context.Context, model.SharedProvider, string) (map[string]bool, error) {
			return nil, errors.New("upstream listing not implemented")
		},
		func(_ context.Context, p model.SharedProvider, _ string) error {
			callCount++
			if p.Model == "gpt-6-luna" {
				return errors.New("HTTP 403")
			}
			return nil
		})
	require.Equal(t, 2, callCount)
	require.Len(t, succeeded, 1)
	require.Equal(t, "gpt-6-sol", succeeded[0].Code)
}
func TestSharedProbeResponseEvidence(t *testing.T) {
	for _, tc := range []struct {
		protocol string
		body     string
		valid    bool
	}{
		{"chat", `{"choices":[{"message":{"content":"pong"}}],"usage":{"prompt_tokens":1}}`, true},
		{"chat", `{"data":[{"id":"deepseek-flash"}]}`, false},
		{"responses", `{"output":[{"type":"message"}],"usage":{"input_tokens":1}}`, true},
		{"responses", `{"output":[],"usage":{"input_tokens":1}}`, false},
		{"anthropic", `{"content":[{"type":"text","text":"pong"}],"usage":{"input_tokens":1}}`, true},
		{"anthropic", `{"content":[],"usage":{"input_tokens":1}}`, false},
	} {
		err := sharedValidateProbeResponse(tc.protocol, 200, []byte(tc.body))
		if tc.valid {
			require.NoError(t, err, tc.protocol)
		} else {
			require.Error(t, err, tc.protocol)
		}
	}
	require.Error(t, sharedValidateProbeResponse("chat", 401, []byte("{}")))
}
