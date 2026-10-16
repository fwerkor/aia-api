package controller

import (
	"bufio"
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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	if !strings.HasSuffix(u.Path, "/v1") {
		return "", errors.New("endpoint must end in /v1")
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	return sharedHTTPClient().Do(req)
}

func SharedProviders(c *gin.Context) {
	var all []model.SharedProvider
	if err := model.DB.Where("enabled = ?", true).Order("id").Find(&all).Error; err != nil {
		shareError(c, 500, "database unavailable")
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
	var account model.SharedAccount
	err := model.DB.First(&account, "user_id = ?", userID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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
	c.JSON(200, gin.H{"success": true, "data": gin.H{"contributions": keys, "balance": account.Balance, "history": history}})
}
func SharedAdminProviders(c *gin.Context) {
	var providers []model.SharedProvider
	if err := model.DB.Order("id").Find(&providers).Error; err != nil {
		shareError(c, 500, "database unavailable")
		return
	}
	c.JSON(200, gin.H{"success": true, "data": providers})
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
	if err != nil {
		shareError(c, 400, err.Error())
		return
	}
	if strings.TrimSpace(p.Model) == "" || len(p.Model) > 100 || len(p.Name) > 100 || p.InputPrice < 0 || p.OutputPrice < 0 ||
		p.InputPrice > 10000 || p.OutputPrice > 10000 || (p.InputPrice == 0 && p.OutputPrice == 0) ||
		math.IsNaN(p.InputPrice) || math.IsNaN(p.OutputPrice) || math.IsInf(p.InputPrice, 0) || math.IsInf(p.OutputPrice, 0) ||
		p.MaxOutput < 1 || p.MaxOutput > 2048 {
		shareError(c, 400, "invalid model, pricing or output bound")
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
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shareError(c, 400, "invalid state")
		return
	}
	result := model.DB.Model(&model.SharedProvider{}).Where("id = ?", id).Update("enabled", body.Enabled)
	if result.Error != nil || result.RowsAffected != 1 {
		shareError(c, 404, "provider not found")
		return
	}
	c.JSON(200, gin.H{"success": true})
}
func SharedAdminGrant(c *gin.Context) {
	var body struct {
		UserID    int    `json:"user_id"`
		Micros    int64  `json:"micros"`
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
	if err := model.SharedGrant(body.UserID, body.Micros, body.Reference); err != nil {
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
		ProviderID uint   `json:"provider_id"`
		Key        string `json:"key"`
		DailyLimit int64  `json:"daily_limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shareError(c, 400, "invalid contribution")
		return
	}
	if body.DailyLimit < 100 || body.DailyLimit > 100000000 || len(body.Key) < 8 || len(body.Key) > 4096 || strings.TrimSpace(body.Key) != body.Key {
		shareError(c, 400, "invalid daily limit or API key")
		return
	}
	var provider model.SharedProvider
	if err := model.DB.First(&provider, "id = ? AND enabled = ?", body.ProviderID, true).Error; err != nil {
		shareError(c, 404, "provider unavailable")
		return
	}
	// A real, low-cost model call checks model-specific permissions. Do not log credentials or upstream content.
	request := map[string]any{"model": provider.Model, "messages": []map[string]string{{"role": "user", "content": "ping"}}, "max_tokens": 1, "stream": false}
	payload, _ := json.Marshal(request)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	resp, err := shareCall(ctx, provider, body.Key, payload)
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
	var validation struct {
		Choices []json.RawMessage `json:"choices"`
		Usage   sharedUsage       `json:"usage"`
	}
	if json.Unmarshal(validationBody, &validation) != nil || len(validation.Choices) == 0 || validation.Usage.PromptTokens < 1 {
		shareError(c, 422, "upstream did not return valid completion usage")
		return
	}
	sealed, fp, err := sharedSeal(body.Key)
	if err != nil {
		shareError(c, 503, "encryption unavailable")
		return
	}
	hint := "****" + body.Key[len(body.Key)-4:]
	item := model.SharedContribution{OwnerID: c.GetInt("id"), ProviderID: body.ProviderID, Ciphertext: sealed, Fingerprint: fp, KeyHint: hint, DailyLimit: body.DailyLimit, Enabled: true, Day: time.Now().UTC().Format("2006-01-02")}
	if err := model.DB.Create(&item).Error; err != nil {
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
	var body struct {
		Enabled    *bool `json:"enabled"`
		DailyLimit int64 `json:"daily_limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		shareError(c, 400, "invalid request")
		return
	}
	if body.DailyLimit < 100 || body.DailyLimit > 100000000 || body.Enabled == nil {
		shareError(c, 400, "invalid budget or status")
		return
	}
	// Reservations retain their own settlement record; disabling only affects new requests.
	res := model.DB.Model(&model.SharedContribution{}).Where("id = ? AND owner_id = ?", id, c.GetInt("id"))
	if *body.Enabled {
		res = res.Where("ciphertext <> ?", "")
	}
	res = res.Updates(map[string]any{"enabled": *body.Enabled, "daily_limit": body.DailyLimit})
	if res.Error != nil || res.RowsAffected != 1 {
		shareError(c, 404, "contribution not found")
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
	// Retain billing history and pending requests; destroy credential only if no active reservations.
	res := model.DB.Model(&model.SharedContribution{}).Where("id = ? AND owner_id = ? AND reserved = 0", id, c.GetInt("id")).
		Updates(map[string]any{"enabled": false, "ciphertext": "", "fingerprint": fmt.Sprintf("revoked:%d", id)})
	if res.Error != nil || res.RowsAffected != 1 {
		shareError(c, 409, "contribution not found or has an active request")
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func SharedModels(c *gin.Context) {
	var all []model.SharedProvider
	model.DB.Where("enabled = ?", true).Find(&all)
	list := make([]gin.H, 0, len(all))
	for _, item := range all {
		list = append(list, gin.H{"id": item.Model, "object": "model", "owned_by": "aia-shared"})
	}
	c.JSON(200, gin.H{"object": "list", "data": list})
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
			return nil, false, 0, errors.New("only text message roles are supported")
		}
	}
	maxTokens := 256
	if provider.MaxOutput < maxTokens {
		maxTokens = provider.MaxOutput
	}
	for _, field := range []string{"max_tokens", "max_completion_tokens"} {
		if x, ok := req[field]; ok {
			var n int
			if err := json.Unmarshal(x, &n); err != nil || n < 1 || n > provider.MaxOutput {
				return nil, false, 0, errors.New("invalid maximum output tokens")
			}
			if n < maxTokens {
				maxTokens = n
			}
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
	estimate := int64(math.Ceil(float64(len(raw)+128)*provider.InputPrice+float64(maxTokens)*provider.OutputPrice)) + 10
	if estimate < 100 {
		estimate = 100
	}
	if estimate > 100000000 {
		return nil, false, 0, errors.New("request is too costly")
	}
	result, err := json.Marshal(req)
	return result, stream, estimate, err
}

type sharedUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func sharedCost(p model.SharedProvider, u sharedUsage, cap int64) int64 {
	if u.PromptTokens < 0 || u.CompletionTokens < 0 {
		return cap
	}
	cost := int64(math.Ceil(float64(u.PromptTokens)*p.InputPrice + float64(u.CompletionTokens)*p.OutputPrice))
	if cost < 1 {
		cost = 1
	}
	if cost > cap {
		cost = cap
	}
	return cost
}
func SharedChat(c *gin.Context) {
	if !shareConfigured(c) {
		return
	}
	// The API key has already passed New API's token authentication.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 65537))
	if err != nil || len(body) > 65536 {
		shareError(c, 413, "request too large")
		return
	}
	var stub struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &stub); err != nil || stub.Model == "" {
		shareError(c, 400, "missing model")
		return
	}
	var provider model.SharedProvider
	if err := model.DB.Where("model = ? AND enabled = ?", stub.Model, true).First(&provider).Error; err != nil {
		shareError(c, 404, "shared model unavailable")
		return
	}
	platformToken, tokenErr := model.GetTokenById(c.GetInt("token_id"))
	if tokenErr != nil {
		shareError(c, 401, "invalid platform token")
		return
	}
	if platformToken.ModelLimitsEnabled {
		allowed := platformToken.GetModelLimitsMap()
		if !allowed[provider.Model] {
			shareError(c, 403, "model is not permitted by platform token")
			return
		}
	}
	payload, stream, maxCost, err := shareBoundedRequest(body, provider)
	if err != nil {
		shareError(c, 400, err.Error())
		return
	}
	model.SharedReclaim()
	requestID := common.NewRequestId()
	contribution, err := model.SharedReserve(c.GetInt("id"), provider.ID, maxCost, requestID)
	if err != nil {
		shareError(c, 402, "insufficient shared credits or all contribution budgets are exhausted")
		return
	}
	settled := false
	// The reservation is refunded on any early failure. Upstream calls have a strict timeout.
	defer func() {
		if !settled {
			_ = model.SharedSettle(requestID, 0)
		}
	}()
	key, err := sharedOpen(contribution.Ciphertext)
	if err != nil {
		shareError(c, 500, "credential unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := shareCall(ctx, provider, key, payload)
	if err != nil {
		shareError(c, 502, "upstream unavailable")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			model.DB.Model(&model.SharedContribution{}).Where("id = ?", contribution.ID).Update("enabled", false)
		} else if resp.StatusCode == http.StatusTooManyRequests {
			model.DB.Model(&model.SharedContribution{}).Where("id = ?", contribution.ID).
				Update("cooldown_until", time.Now().Add(time.Minute).Unix())
		}
		// No upstream response usage can be verified: no reward is minted.
		shareError(c, 502, "upstream error (HTTP "+strconv.Itoa(resp.StatusCode)+")")
		return
	}
	if !stream {
		content, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			shareError(c, 502, "incomplete upstream response")
			return
		}
		var parsed struct {
			Usage   sharedUsage       `json:"usage"`
			Choices []json.RawMessage `json:"choices"`
		}
		if json.Unmarshal(content, &parsed) != nil || len(parsed.Choices) == 0 || parsed.Usage.PromptTokens <= 0 {
			shareError(c, 502, "upstream did not return auditable token usage")
			return
		}
		if err := model.SharedSettle(requestID, sharedCost(provider, parsed.Usage, maxCost)); err != nil {
			shareError(c, 500, "settlement failed")
			return
		}
		settled = true
		c.Data(200, "application/json", content)
		return
	}
	// Forward SSE while collecting an upstream-provided usage event. When usage is absent,
	// debit the maximum reservation rather than allowing free streamed traffic.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)
	reader := bufio.NewReader(resp.Body)
	var usage sharedUsage
	usageSeen := false
	bytesSent := 0
	clientDisconnected := false
	for {
		line, readErr := reader.ReadBytes(10)
		bytesSent += len(line)
		if bytesSent > 5<<20 {
			break
		}
		if bytes.HasPrefix(line, []byte("data: ")) {
			var event struct {
				Usage *sharedUsage `json:"usage"`
			}
			if json.Unmarshal(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data: "))), &event) == nil && event.Usage != nil && event.Usage.PromptTokens > 0 {
				usage = *event.Usage
				usageSeen = true
			}
		}
		if len(line) > 0 {
			if !clientDisconnected {
				if _, writeErr := c.Writer.Write(line); writeErr == nil {
					c.Writer.Flush()
				} else {
					clientDisconnected = true
				}
			}
		}
		if readErr != nil {
			break
		}
	}
	// An unknown upstream charge must not create phantom earned credits.
	if usageSeen {
		err = model.SharedSettle(requestID, sharedCost(provider, usage, maxCost))
	} else {
		err = model.SharedSettle(requestID, maxCost)
	}
	if err == nil {
		settled = true
	}
}
