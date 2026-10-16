package controller

import (
	"encoding/hex"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSharedCredentialCrypto(t *testing.T) {
	t.Setenv("AIA_SHARED_SECRET", hex.EncodeToString(make([]byte, 32)))
	encrypted, fp, err := sharedSeal("sk-test-contributed-secret")
	require.NoError(t, err)
	require.NotContains(t, encrypted, "secret")
	require.Len(t, fp, 64)
	decrypted, err := sharedOpen(encrypted)
	require.NoError(t, err)
	require.Equal(t, "sk-test-contributed-secret", decrypted)
	encrypted2, fp2, err := sharedSeal("sk-test-contributed-secret")
	require.NoError(t, err)
	require.NotEqual(t, encrypted, encrypted2)
	require.Equal(t, fp, fp2)
	t.Setenv("AIA_SHARED_SECRET", hex.EncodeToString(make([]byte, 32))+"AA")
	_, err = sharedOpen(encrypted)
	require.Error(t, err)
}
func TestSharedRequestBounds(t *testing.T) {
	provider := model.SharedProvider{Model: "test-model", InputPrice: 0.25, OutputPrice: 1.5, MaxOutput: 512}
	_, stream, limit, err := shareBoundedRequest([]byte(`{"model":"test-model","messages":[{"role":"user","content":"hi"}],"max_tokens":16}`), provider)
	require.NoError(t, err)
	require.False(t, stream)
	require.Greater(t, limit, int64(0))
	forwarded, _, _, err := shareBoundedRequest([]byte(`{"model":"test-model","messages":[{"role":"user","content":"hi","image_url":"http://internal.example/image"}],"max_tokens":400}`), provider)
	require.NoError(t, err)
	require.NotContains(t, string(forwarded), "image_url")
	require.Contains(t, string(forwarded), `"max_tokens":400`)
	for _, input := range []string{
		`{"model":"test-model","messages":[{"role":"user","content":"hi"}],"tools":[]}`,
		`{"model":"test-model","messages":[{"role":"user","content":"hi"}],"max_tokens":1000000}`,
		`{"model":"another","messages":[{"role":"user","content":"hi"}]}`,
		`{"model":"test-model","messages":[{"role":"user","content":[{"type":"image_url"}]}]}`,
	} {
		_, _, _, err = shareBoundedRequest([]byte(input), provider)
		require.Error(t, err)
	}
}
func TestSharedEndpointValidation(t *testing.T) {
	for _, host := range []string{"http://api.example.com/v1", "https://127.0.0.1/v1", "https://example.com/x", "https://username:pass@example.com/v1"} {
		_, err := shareEndpoint(host)
		require.Error(t, err)
	}
	u, err := shareEndpoint("https://api.example.com/v1/")
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/v1", u)
}
