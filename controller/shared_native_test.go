package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSharedResponsesBudgetValidation(t *testing.T) {
	p := model.SharedProvider{Protocol: "responses", Model: "gpt-6-sol", InputPrice: 2, OutputPrice: 10, MaxOutput: 512}
	price, err := shareBoundedResponses([]byte(`{"model":"gpt-6-sol","input":"Hello","max_output_tokens":128}`), p)
	require.NoError(t, err)
	require.Positive(t, price)
	for _, body := range []string{
		`{"model":"gpt-6-sol","input":"Hi","max_output_tokens":2048}`,
		`{"model":"gpt-6-sol","input":"Hi","tools":[{"type":"web_search"}],"max_output_tokens":128}`,
		`{"model":"gpt-6-sol","input":[{"role":"user","content":[{"type":"input_image","image_url":"https://x"}]}],"max_output_tokens":128}`,
		`{"model":"gpt-6-sol","input":"Hi","max_output_tokens":128,"background":true}`,
		`{"model":"gpt-6-sol","input":"Hi"}`,
		`{"model":"gpt-6-sol","input":"Hi","max_output_tokens":128,"instructions":{"foo":"bar"}}`,
	} {
		_, err = shareBoundedResponses([]byte(body), p)
		require.Error(t, err, body)
	}
}
