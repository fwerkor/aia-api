package controller

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func sharedSecret() ([]byte, error) {
	key, err := hex.DecodeString(os.Getenv("AIA_SHARED_SECRET"))
	if err != nil || len(key) != 32 {
		return nil, errors.New("AIA_SHARED_SECRET must contain 64 hex characters")
	}
	return key, nil
}
func sharedSeal(plaintext string) (string, string, error) {
	key, err := sharedSecret()
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(plaintext))
	return base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), hex.EncodeToString(mac.Sum(nil)), nil
}
func sharedOpen(ciphertext string) (string, error) {
	key, err := sharedSecret()
	if err != nil {
		return "", err
	}
	data, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted credential")
	}
	plaintext, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	return string(plaintext), err
}
func shareError(c *gin.Context, status int, detail string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"type": "shared_pool_error", "message": detail}})
}
func shareConfigured(c *gin.Context) bool {
	if _, err := sharedSecret(); err != nil {
		shareError(c, http.StatusServiceUnavailable, "shared pool is not configured")
		return false
	}
	return true
}

func shareEndpoint(raw string) (string, error) {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("endpoint must be a plain HTTPS URL")
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return "", errors.New("local endpoints are disallowed")
	}
	if ip := net.ParseIP(host); ip != nil && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback()) {
		return "", errors.New("private endpoint is disallowed")
	}
	if !strings.HasSuffix(u.Path, "/v1") && u.Path != "/v1beta/openai" {
		return "", errors.New("unsupported endpoint base path")
	}
	return raw, nil
}
func sharedHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() {
				continue
			}
			d := net.Dialer{Timeout: 8 * time.Second}
			if conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port)); err == nil {
				return conn, nil
			}
		}
		return nil, errors.New("no public address available for upstream")
	}
	return &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func shareCall(ctx context.Context, p model.SharedProvider, key string, body []byte) (*http.Response, error) {
	protocol := model.SharedProtocol(p.Protocol)
	path := "/chat/completions"
	if protocol == "responses" {
		path = "/responses"
		body, _ = json.Marshal(map[string]any{"model": p.Model, "input": "ping", "max_output_tokens": 32})
	} else if protocol == "anthropic" {
		path = "/messages"
		body, _ = json.Marshal(map[string]any{"model": p.Model, "messages": []map[string]string{{"role": "user", "content": "ping"}}, "max_tokens": 16})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if protocol == "anthropic" {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	return sharedHTTPClient().Do(req)
}

func SharedProviders(c *gin.Context) {
	all, err := model.SharedProvidersWithModels(true)
	if err != nil {
		shareError(c, 500, "providers unavailable")
		return
	}
	c.JSON(200, gin.H{"success": true, "data": all})
}
func SharedMine(c *gin.Context) {
	if !shareConfigured(c) {
		return
	}
	userID := c.GetInt("id")
	var keys []model.SharedContribution
	if err := model.DB.Where("owner_id = ?", userID).Order("id desc").Find(&keys).Error; err != nil {
		shareError(c, 500, "database unavailable")
		return
	}
	balance, err := model.GetUserQuota(userID, false)
	if err != nil {
		shareError(c, 500, "database unavailable")
		return
	}
	var charges []model.SharedCharge
	model.DB.Where("consumer_id = ? OR owner_id = ?", userID, userID).Order("id desc").Limit(50).Find(&charges)
	history := make([]gin.H, 0, len(charges))
	for _, ch := range charges {
		direction := "consume"
		if ch.Status == "grant" {
			direction = "grant"
		} else if ch.ConsumerID == ch.OwnerID {
			direction = "self"
		} else if ch.OwnerID == userID {
			direction = "earn"
		}
		history = append(history, gin.H{"request_id": ch.RequestID, "direction": direction, "charged": ch.Charged, "status": ch.Status, "created_at": ch.CreatedAt})
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"contributions": keys, "balance": balance, "history": history}})
}
func SharedAdminProviders(c *gin.Context) {
	all, err := model.SharedProvidersWithModels(false)
	if err != nil {
		shareError(c, 500, "providers unavailable")
		return
	}
	c.JSON(200, gin.H{"success": true, "data": all})
}
func SharedAdminCreateProvider(c *gin.Context) {
	if !shareConfigured(c) {
		return
	}
	var p model.SharedProvider
	if err := c.ShouldBindJSON(&p); err != nil {
		shareError(c, 400, "invalid provider")
		return
	}
	endpoint, err := shareEndpoint(p.BaseURL)
	p.Protocol = model.SharedProtocol(p.Protocol)
	if p.Protocol != "chat" && p.Protocol != "responses" && p.Protocol != "anthropic" && p.Protocol != "gemini" {
		shareError(c, 400, "unsupported API protocol")
		return
	}
	if err == nil {
		u, _ := url.Parse(endpoint)
		if (p.Protocol == "gemini" && u.Path != "/v1beta/openai") ||
			(p.Protocol != "gemini" && !strings.HasSuffix(u.Path, "/v1")) {
			err = errors.New("base URL does not match protocol")
		}
	}
	if err != nil {
		shareError(c, 400, err.Error())
		return
	}
	if len(strings.TrimSpace(p.Name)) == 0 || len(p.Name) > 120 {
		shareError(c, 400, "invalid endpoint name")
		return
	}
	p.BaseURL = endpoint
	p.ID = 0
	p.Enabled = false // admin enables only after checking contract and pricing
	if err := model.DB.Create(&p).Error; err != nil {
		shareError(c, 409, "model already registered or database error")
		return
	}
	c.JSON(200, gin.H{"success": true, "data": p})
}
func SharedAdminEnableProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shareError(c, 400, "invalid id")
		return
	}
	var body struct {
		Enabled     bool     `json:"enabled"`
		InputPrice  *float64 `json:"input_price"`
		OutputPrice *float64 `json:"output_price"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shareError(c, 400, "invalid state")
		return
	}
	if body.InputPrice != nil || body.OutputPrice != nil {
		if body.InputPrice == nil || body.OutputPrice == nil || *body.InputPrice <= 0 || *body.OutputPrice <= 0 || *body.InputPrice > 10000 || *body.OutputPrice > 10000 ||
			math.IsNaN(*body.InputPrice) || math.IsNaN(*body.OutputPrice) || math.IsInf(*body.InputPrice, 0) || math.IsInf(*body.OutputPrice, 0) {
			shareError(c, 400, "both verified upstream prices required")
			return
		}
		if err := model.DB.Model(&model.SharedProvider{}).Where("id = ?", id).
			Updates(map[string]any{"input_price": *body.InputPrice, "output_price": *body.OutputPrice}).Error; err != nil {
			shareError(c, 500, "unable to update endpoint prices")
			return
		}
	}
	if body.Enabled {
		var p model.SharedProvider
		if err := model.DB.First(&p, id).Error; err != nil {
			shareError(c, 404, "provider not found")
			return
		}
		bindings, err := model.SharedProviderModels(p.ID, true)
		if err != nil || len(bindings) == 0 {
			shareError(c, 400, "link at least one active model")
			return
		}
		for _, m := range bindings {
			if m.InputPrice <= 0 || m.OutputPrice <= 0 || m.MaxOutput <= 0 {
				shareError(c, 400, "verify catalog model pricing before enabling")
				return
			}
			nativePrice, hasPrice := ratio_setting.GetModelPrice(m.Code, false)
			nativeRatio, hasRatio, _ := ratio_setting.GetModelRatio(m.Code)
			if (!hasPrice || nativePrice <= 0) && (!hasRatio || nativeRatio <= 0) {
				shareError(c, 400, "configure New API native price for model: "+m.Code)
				return
			}
		}
	}
	if err := model.SharedSetProviderEnabled(uint(id), body.Enabled); err != nil {
		shareError(c, 404, "provider not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}
func SharedAdminGrant(c *gin.Context) {
	var body struct {
		UserID    int    `json:"user_id"`
		Quota     int64  `json:"quota"`
		Reference string `json:"reference"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shareError(c, 400, "invalid grant")
		return
	}
	if len(body.Reference) == 0 || len(body.Reference) > 64 {
		shareError(c, 400, "invalid grant reference")
		return
	}
	var target model.User
	if err := model.DB.Select("id").First(&target, "id = ?", body.UserID).Error; err != nil {
		shareError(c, 404, "user not found")
		return
	}
	if err := model.SharedGrant(body.UserID, body.Quota, body.Reference); err != nil {
		shareError(c, 400, "grant rejected (possibly duplicate reference)")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func SharedContribute(c *gin.Context) {
	if !shareConfigured(c) {
		return
	}
	var body struct {
		ProviderID uint     `json:"provider_id"`
		Key        string   `json:"key"`
		DailyLimit int64    `json:"daily_limit"`
		Models     []string `json:"models"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shareError(c, 400, "invalid contribution")
		return
	}
	if body.DailyLimit < 50 || body.DailyLimit > 50000000 || len(body.Key) < 8 || len(body.Key) > 4096 || strings.TrimSpace(body.Key) != body.Key {
		shareError(c, 400, "invalid daily limit or API key")
		return
	}
	var provider model.SharedProvider
	if err := model.DB.First(&provider, "id = ? AND enabled = ?", body.ProviderID, true).Error; err != nil {
		shareError(c, 404, "provider unavailable")
		return
	}
	selected, err := model.SharedVerifyModels(provider.ID, body.Models)
	if err != nil {
		shareError(c, 400, err.Error())
		return
	}
	for _, selectedModel := range selected {
		quote := provider
		quote.Model = selectedModel.Code
		quote.InputPrice = selectedModel.InputPrice
		quote.OutputPrice = selectedModel.OutputPrice
		quote.MaxOutput = selectedModel.MaxOutput
		// A real, low-cost model call checks model-specific permissions. Do not log credentials or upstream content.
		request := map[string]any{"model": quote.Model, "messages": []map[string]string{{"role": "user", "content": "ping"}}, "max_tokens": 1, "stream": false}
		payload, _ := json.Marshal(request)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		resp, err := shareCall(ctx, quote, body.Key, payload)
		if err != nil {
			shareError(c, 422, "endpoint validation failed")
			return
		}
		validationBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8192))
		resp.Body.Close()
		if readErr != nil {
			shareError(c, 422, "endpoint validation failed")
			return
		}
		if resp.StatusCode != 200 {
			shareError(c, 422, "key cannot call the configured model (upstream HTTP "+strconv.Itoa(resp.StatusCode)+")")
			return
		}
		var response map[string]json.RawMessage
		if json.Unmarshal(validationBody, &response) != nil {
			shareError(c, 422, "invalid upstream response")
			return
		}
		switch model.SharedProtocol(quote.Protocol) {
		case "responses":
			var rawUsage map[string]int
			if json.Unmarshal(response["usage"], &rawUsage) != nil || rawUsage["input_tokens"] <= 0 || len(response["output"]) == 0 {
				shareError(c, 422, "Responses API did not return auditable usage")
				return
			}
		case "anthropic":
			var usage map[string]int
			if json.Unmarshal(response["usage"], &usage) != nil || usage["input_tokens"] <= 0 || len(response["content"]) == 0 {
				shareError(c, 422, "Anthropic API did not return auditable usage")
				return
			}
		default:
			var usage map[string]int
			var choices []json.RawMessage
			if json.Unmarshal(response["usage"], &usage) != nil || usage["prompt_tokens"] <= 0 || json.Unmarshal(response["choices"], &choices) != nil || len(choices) == 0 {
				shareError(c, 422, "Chat API did not return auditable usage")
				return
			}
		}
	}
	sealed, fp, err := sharedSeal(body.Key)
	if err != nil {
		shareError(c, 503, "encryption unavailable")
		return
	}
	hint := "****" + body.Key[len(body.Key)-4:]
	item := model.SharedContribution{OwnerID: c.GetInt("id"), ProviderID: body.ProviderID, Models: strings.Join(body.Models, ","), Ciphertext: sealed, Fingerprint: fp, KeyHint: hint, DailyLimit: body.DailyLimit, Enabled: true, Day: time.Now().UTC().Format("2006-01-02")}
	if err := model.SharedCreateContribution(&item, provider); err != nil {
		shareError(c, 409, "key already contributed or database error")
		return
	}
	c.JSON(200, gin.H{"success": true, "data": item})
}
func SharedUpdateContribution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shareError(c, 400, "invalid contribution")
		return
	}
	var req struct {
		Enabled    *bool `json:"enabled"`
		DailyLimit int64 `json:"daily_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil || req.DailyLimit < 50 || req.DailyLimit > 50000000 {
		shareError(c, 400, "invalid budget or state")
		return
	}
	if err := model.SharedSetEnabled(c.GetInt("id"), uint(id), *req.Enabled, req.DailyLimit, false); err != nil {
		shareError(c, 409, "contribution cannot be updated")
		return
	}
	c.JSON(200, gin.H{"success": true})
}
func SharedDeleteContribution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		shareError(c, 400, "invalid contribution")
		return
	}
	var item model.SharedContribution
	if err := model.DB.Where("id = ? AND owner_id = ?", id, c.GetInt("id")).First(&item).Error; err != nil {
		shareError(c, 404, "contribution not found")
		return
	}
	if err := model.SharedSetEnabled(c.GetInt("id"), uint(id), false, item.DailyLimit, true); err != nil {
		shareError(c, 409, "contribution has active calls; pause first")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

type sharedUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func shareBoundedRequest(raw []byte, provider model.SharedProvider) ([]byte, bool, int64, error) {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, false, 0, errors.New("invalid JSON")
	}
	// Only text messages and ordinary chat parameters are accepted: no tools, images, audio, URLs or uncontrolled compute.
	allowed := map[string]bool{"model": true, "messages": true, "max_tokens": true, "max_completion_tokens": true, "stream": true, "temperature": true, "top_p": true, "stop": true, "seed": true, "presence_penalty": true, "frequency_penalty": true}
	for k := range req {
		if !allowed[k] {
			return nil, false, 0, fmt.Errorf("unsupported field: %s", k)
		}
	}
	var modelName string
	if err := json.Unmarshal(req["model"], &modelName); err != nil || modelName != provider.Model {
		return nil, false, 0, errors.New("model is not in shared pool")
	}
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(req["messages"], &msgs); err != nil || len(msgs) == 0 || len(msgs) > 64 {
		return nil, false, 0, errors.New("text messages required")
	}
	for _, m := range msgs {
		if (m.Role != "system" && m.Role != "user" && m.Role != "assistant") || len(m.Content) > 32768 {
			return nil, false, 0, errors.New("only text messages are supported")
		}
	}
	var rawMessages []map[string]json.RawMessage
	if err := json.Unmarshal(req["messages"], &rawMessages); err != nil {
		return nil, false, 0, err
	}
	for _, msg := range rawMessages {
		for k := range msg {
			if k != "role" && k != "content" {
				return nil, false, 0, errors.New("unsupported nested message field")
			}
		}
		if _, ok := msg["role"]; !ok {
			return nil, false, 0, errors.New("missing role")
		}
		if _, ok := msg["content"]; !ok {
			return nil, false, 0, errors.New("missing content")
		}
	}
	// Strip all extraneous nested message fields before forwarding upstream.
	req["messages"], _ = json.Marshal(msgs)
	maxTokens := provider.MaxOutput
	if _, old := req["max_tokens"]; !old {
		if _, newCap := req["max_completion_tokens"]; !newCap {
			return nil, false, 0, errors.New("specify max_tokens or max_completion_tokens for shared channels")
		}
	}
	for _, field := range []string{"max_tokens", "max_completion_tokens"} {
		if x, ok := req[field]; ok {
			var n int
			if err := json.Unmarshal(x, &n); err != nil || n < 1 || n > provider.MaxOutput {
				return nil, false, 0, errors.New("invalid maximum output tokens")
			}
			maxTokens = n
		}
	}
	// Reject inconsistent dual bounds and forcibly set an output bound.
	_, useNewBound := req["max_completion_tokens"]
	if _, alsoOld := req["max_tokens"]; alsoOld && useNewBound {
		return nil, false, 0, errors.New("specify only one output bound")
	}
	delete(req, "max_tokens")
	delete(req, "max_completion_tokens")
	if useNewBound {
		req["max_completion_tokens"], _ = json.Marshal(maxTokens)
	} else {
		req["max_tokens"], _ = json.Marshal(maxTokens)
	}
	var stream bool
	if v, ok := req["stream"]; ok {
		if err := json.Unmarshal(v, &stream); err != nil {
			return nil, false, 0, errors.New("invalid stream field")
		}
	}
	if stream {
		req["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	}
	// Upper-bound input by UTF-8 request bytes; tokens cannot materially exceed byte length.
	estimate := int64(math.Ceil(float64(len(raw)+1024)*provider.InputPrice*2+float64(maxTokens)*provider.OutputPrice*3)) + 10
	if estimate < 100 {
		estimate = 100
	}
	if estimate > 100000000 {
		return nil, false, 0, errors.New("request is too costly")
	}
	result, err := json.Marshal(req)
	return result, stream, estimate, err
}
