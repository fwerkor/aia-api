package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

type SharedProbeResult struct {
	Code     string `json:"code"`
	Verified bool   `json:"verified"`
	Reason   string `json:"reason,omitempty"`
}

func sharedProbeCatalog(provider model.SharedProvider, codes []string) ([]model.SharedModel, error) {
	if len(codes) > 8 {
		return nil, errors.New("at most eight models per probe")
	}
	available, err := model.SharedModelCandidates(provider.ID, provider.Protocol)
	if err != nil {
		return nil, err
	}
	if len(codes) == 0 {
		return available, nil
	}
	selected := make([]model.SharedModel, 0, len(codes))
	seen := map[string]bool{}
	for _, code := range codes {
		if seen[code] {
			return nil, errors.New("duplicate model")
		}
		seen[code] = true
		matched := false
		for _, m := range available {
			if code == m.Code {
				selected = append(selected, m)
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("model is not eligible for testing: %s", code)
		}
	}
	return selected, nil
}

// Listing reduces probe calls, but models reported by /models are never
// accepted without a successfully metered completion.
func sharedListUpstreamModels(ctx context.Context, p model.SharedProvider, key string) (map[string]bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if model.SharedProtocol(p.Protocol) == "anthropic" {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := sharedHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model listing HTTP %d", resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}
	var data struct{ Data []struct{ ID string } }
	if json.Unmarshal(payload, &data) != nil || len(data.Data) == 0 {
		return nil, errors.New("upstream does not provide a model list")
	}
	found := map[string]bool{}
	for _, item := range data.Data {
		if item.ID != "" {
			found[item.ID] = true
		}
	}
	return found, nil
}

func sharedValidateProbeResponse(protocol string, status int, body []byte) error {
	if status != http.StatusOK {
		return fmt.Errorf("upstream HTTP %d", status)
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(body, &result) != nil {
		return errors.New("invalid model response")
	}
	switch model.SharedProtocol(protocol) {
	case "responses":
		var usage map[string]int
		var output []json.RawMessage
		if json.Unmarshal(result["usage"], &usage) != nil || usage["input_tokens"] <= 0 ||
			json.Unmarshal(result["output"], &output) != nil || len(output) == 0 {
			return errors.New("response usage/output missing")
		}
	case "anthropic":
		var usage map[string]int
		var content []json.RawMessage
		if json.Unmarshal(result["usage"], &usage) != nil || usage["input_tokens"] <= 0 ||
			json.Unmarshal(result["content"], &content) != nil || len(content) == 0 {
			return errors.New("message usage/content missing")
		}
	default:
		var usage map[string]int
		var choices []json.RawMessage
		if json.Unmarshal(result["usage"], &usage) != nil || usage["prompt_tokens"] <= 0 ||
			json.Unmarshal(result["choices"], &choices) != nil || len(choices) == 0 {
			return errors.New("completion usage/choices missing")
		}
	}
	return nil
}

func sharedVerifyProbe(ctx context.Context, p model.SharedProvider, key string) error {
	request := map[string]any{
		"model":      p.Model,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 16,
		"stream":     false,
	}
	payload, _ := json.Marshal(request)
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	resp, err := shareCall(ctx, p, key, payload)
	if err != nil {
		return errors.New("upstream connection failed")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return errors.New("upstream response unreadable")
	}
	return sharedValidateProbeResponse(p.Protocol, resp.StatusCode, raw)
}

// Caller controls a bounded list of existing catalog models. /models results
// are an optimization; the authenticated completion is authoritative.
func sharedProbeWith(ctx context.Context, p model.SharedProvider, key string, candidates []model.SharedModel,
	list func(context.Context, model.SharedProvider, string) (map[string]bool, error),
	verify func(context.Context, model.SharedProvider, string) error) ([]model.SharedModel, []SharedProbeResult) {
	listed, listErr := list(ctx, p, key)
	useListing := listErr == nil && len(listed) > 0
	if useListing {
		hasCandidate := false
		for _, candidate := range candidates {
			if listed[candidate.Code] {
				hasCandidate = true
				break
			}
		}
		if !hasCandidate {
			useListing = false
		}
	}
	verified := make([]model.SharedModel, 0, len(candidates))
	results := make([]SharedProbeResult, 0, len(candidates))
	for _, m := range candidates {
		if ctx.Err() != nil {
			results = append(results, SharedProbeResult{Code: m.Code, Reason: "probe timeout"})
			break
		}
		if useListing && !listed[m.Code] {
			results = append(results, SharedProbeResult{Code: m.Code, Reason: "not listed by upstream"})
			continue
		}
		quote := p
		quote.Model = m.Code
		quote.InputPrice = m.InputPrice
		quote.OutputPrice = m.OutputPrice
		quote.MaxOutput = m.MaxOutput
		err := verify(ctx, quote, key)
		if err != nil {
			reason := err.Error()
			if strings.Contains(reason, "HTTP 401") || strings.Contains(reason, "HTTP 403") {
				results = append(results, SharedProbeResult{Code: m.Code, Reason: "API key unauthorized"})
				break
			}
			results = append(results, SharedProbeResult{Code: m.Code, Reason: reason})
			continue
		}
		verified = append(verified, m)
		results = append(results, SharedProbeResult{Code: m.Code, Verified: true})
	}
	return verified, results
}
