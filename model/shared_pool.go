package model

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SharedProvider struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	Name        string  `json:"name"`
	BaseURL     string  `json:"base_url"`
	Protocol    string  `json:"protocol" gorm:"default:chat"`
	Model       string  `json:"model" gorm:"uniqueIndex"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
	MaxOutput   int     `json:"max_output"`
	Enabled     bool    `json:"enabled"`
}

type SharedContribution struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	OwnerID       int       `json:"owner_id" gorm:"index"`
	ProviderID    uint      `json:"provider_id" gorm:"index"`
	ChannelID     int       `json:"channel_id" gorm:"index"`
	Ciphertext    string    `json:"-" gorm:"type:text"`
	Fingerprint   string    `json:"-" gorm:"uniqueIndex"`
	KeyHint       string    `json:"key_hint"`
	Enabled       bool      `json:"enabled"`
	CooldownUntil int64     `json:"cooldown_until"`
	DailyLimit    int64     `json:"daily_limit"`
	DailyUsed     int64     `json:"daily_used"`
	Reserved      int64     `json:"reserved"`
	Day           string    `json:"day"`
	CreatedAt     time.Time `json:"created_at"`
}

// SharedCharge is only an idempotent allocation/reward log. users.quota is the one wallet.
type SharedCharge struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	RequestID      string    `json:"request_id" gorm:"uniqueIndex"`
	ConsumerID     int       `json:"consumer_id" gorm:"index"`
	OwnerID        int       `json:"owner_id" gorm:"index"`
	ContributionID uint      `json:"contribution_id"`
	Reserved       int64     `json:"reserved"`
	Charged        int64     `json:"charged"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at" gorm:"index"`
}

var ErrSharedInsufficient = errors.New("contributed channel budget exhausted")

func SharedMigrate() error {
	if err := DB.AutoMigrate(&SharedProvider{}, &SharedContribution{}, &SharedCharge{}); err != nil {
		return err
	}
	return SharedSeedProviderTemplates()
}

// Official API-key gateways. Deliberately disabled until upstream consent and
// platform pricing are reviewed. Subscription/OAuth quotas are not transferable.
func SharedSeedProviderTemplates() error {
	templates := []SharedProvider{
		{Name: "OpenCode Zen API Key（非订阅）", BaseURL: "https://opencode.ai/zen/v1", Protocol: "chat", Model: "kimi-k2.5", MaxOutput: 512},
		{Name: "Codex 开发者 API（非 ChatGPT 订阅）", BaseURL: "https://api.openai.com/v1", Protocol: "responses", Model: "gpt-6-sol", InputPrice: 2, OutputPrice: 10, MaxOutput: 512},
		{Name: "Claude Code 开发者 API（非 Claude 订阅）", BaseURL: "https://api.anthropic.com/v1", Protocol: "anthropic", Model: "claude-sonnet-4-6", InputPrice: 3, OutputPrice: 15, MaxOutput: 512},
		{Name: "Antigravity 相关 Gemini API（非 IDE 配额）", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", Protocol: "gemini", Model: "gemini-3.8-flash", InputPrice: 0.75, OutputPrice: 3.75, MaxOutput: 512},
	}
	for _, tpl := range templates {
		var found SharedProvider
		err := DB.Where("model = ?", tpl.Model).First(&found).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := DB.Create(&tpl).Error; err != nil {
			return err
		}
	}
	return nil
}
func SharedProtocol(p string) string {
	if p == "" {
		return "chat"
	}
	return p
}
func SharedChannelSupportsPath(channelID int, path string) bool {
	var item SharedContribution
	if err := DB.Select("provider_id").Where("channel_id = ?", channelID).First(&item).Error; err != nil {
		return false
	}
	var p SharedProvider
	if err := DB.Select("protocol").First(&p, item.ProviderID).Error; err != nil {
		return false
	}
	if SharedProtocol(p.Protocol) == "responses" {
		return path == "/v1/responses"
	}
	return path == "/v1/chat/completions"
}

func SharedKeyMarker(id uint) string { return fmt.Sprintf("aia-shared:%d", id) }

func SharedIsChannel(channel *Channel) bool {
	return channel != nil && strings.HasPrefix(channel.Key, "aia-shared:")
}

func SharedDecryptCredential(channelID int) (string, error) {
	var contribution SharedContribution
	if err := DB.Where("channel_id = ? AND enabled = ? AND ciphertext <> ?", channelID, true, "").First(&contribution).Error; err != nil {
		return "", err
	}
	if contribution.CooldownUntil > time.Now().Unix() {
		return "", ErrSharedInsufficient
	}
	raw, err := hex.DecodeString(os.Getenv("AIA_SHARED_SECRET"))
	if err != nil || len(raw) != 32 {
		return "", errors.New("shared key encryption is not configured")
	}
	cipherData, err := base64.RawURLEncoding.DecodeString(contribution.Ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(cipherData) < g.NonceSize() {
		return "", errors.New("invalid encrypted shared key")
	}
	key, err := g.Open(nil, cipherData[:g.NonceSize()], cipherData[g.NonceSize():], nil)
	return string(key), err
}

func SharedCreateContribution(item *SharedContribution, provider SharedProvider) error {
	if item.OwnerID <= 0 || item.DailyLimit <= 0 || item.Ciphertext == "" {
		return errors.New("invalid contribution")
	}
	base := strings.TrimSuffix(provider.BaseURL, "/v1")
	channelType := constant.ChannelTypeOpenAI
	switch SharedProtocol(provider.Protocol) {
	case "chat", "responses":
	case "anthropic":
		channelType = constant.ChannelTypeAnthropic
	case "gemini":
		channelType = constant.ChannelTypeGemini
		base = "https://generativelanguage.googleapis.com"
	default:
		return errors.New("unsupported provider protocol")
	}
	channel := &Channel{Type: channelType, Key: "aia-shared:pending",
		Status: common.ChannelStatusEnabled, Name: fmt.Sprintf("AIA shared %s · user %d", provider.Model, item.OwnerID),
		CreatedTime: time.Now().Unix(), BaseURL: &base, Models: provider.Model, Group: "default"}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		channel.Key = SharedKeyMarker(item.ID)
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		item.ChannelID = channel.Id
		if err := tx.Model(item).Update("channel_id", channel.Id).Error; err != nil {
			return err
		}
		return channel.AddAbilities(tx)
	})
	if err == nil {
		InitChannelCache()
	}
	return err
}

func SharedSetEnabled(ownerID int, id uint, enabled bool, dailyLimit int64, revoke bool) error {
	if ownerID <= 0 || dailyLimit <= 0 {
		return errors.New("invalid contribution settings")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var item SharedContribution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", id, ownerID).First(&item).Error; err != nil {
			return err
		}
		if revoke && item.Reserved != 0 {
			return ErrSharedInsufficient
		}
		if enabled && item.Ciphertext == "" {
			return errors.New("revoked credentials cannot be reenabled")
		}
		fields := map[string]any{"enabled": enabled, "daily_limit": dailyLimit}
		if revoke {
			fields["ciphertext"] = ""
			fields["fingerprint"] = fmt.Sprintf("revoked:%d", id)
		}
		if err := tx.Model(&SharedContribution{}).Where("id = ?", id).Updates(fields).Error; err != nil {
			return err
		}
		status := common.ChannelStatusManuallyDisabled
		if enabled {
			status = common.ChannelStatusEnabled
		}
		if err := tx.Model(&Channel{}).Where("id = ?", item.ChannelID).Update("status", status).Error; err != nil {
			return err
		}
		return tx.Model(&Ability{}).Where("channel_id = ?", item.ChannelID).Update("enabled", enabled).Error
	})
	if err == nil {
		InitChannelCache()
	}
	return err
}

func SharedReserveChannel(channelID, userID int, requestID string, amount int64) (bool, error) {
	if amount <= 0 || amount > int64(^uint(0)>>1) {
		return false, errors.New("invalid budget")
	}
	var found bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var item SharedContribution
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("channel_id = ?", channelID).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if !item.Enabled || item.Ciphertext == "" || item.CooldownUntil > time.Now().Unix() {
			return ErrSharedInsufficient
		}
		today := time.Now().UTC().Format("2006-01-02")
		if item.Day != today {
			item.Day = today
			item.DailyUsed = 0
		}
		if item.DailyUsed+item.Reserved+amount > item.DailyLimit {
			return ErrSharedInsufficient
		}
		item.Reserved += amount
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		return tx.Create(&SharedCharge{RequestID: requestID, ConsumerID: userID, OwnerID: item.OwnerID, ContributionID: item.ID, Reserved: amount, Status: "reserved", CreatedAt: time.Now().UTC()}).Error
	})
	return found, err
}

// Settle after native consumption was successful. The contributor's users.quota
// increment and unique settlement record commit atomically in the same database.
func SharedSettleChannel(requestID string, actual int64) error {
	if actual < 0 {
		return errors.New("negative quota")
	}
	var credited int64
	var owner int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var charge SharedCharge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_id = ?", requestID).First(&charge).Error; err != nil {
			return err
		}
		if charge.Status != "reserved" {
			return nil
		}
		var item SharedContribution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, charge.ContributionID).Error; err != nil {
			return err
		}
		if actual > charge.Reserved {
			actual = charge.Reserved
		}
		item.Reserved -= charge.Reserved
		if item.Reserved < 0 {
			return errors.New("invalid reservation state")
		}
		if item.Day == charge.CreatedAt.UTC().Format("2006-01-02") {
			item.DailyUsed += actual
		}
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		charge.Charged = actual
		charge.Status = "settled"
		if err := tx.Save(&charge).Error; err != nil {
			return err
		}
		if actual > 0 {
			result := tx.Model(&User{}).Where("id = ?", charge.OwnerID).Update("quota", gorm.Expr("quota + ?", actual))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("contributor no longer exists")
			}
			credited = actual
			owner = charge.OwnerID
		}
		return nil
	})
	if err == nil && credited > 0 {
		if cacheErr := cacheIncrUserQuota(owner, credited); cacheErr != nil {
			common.SysLog("shared contributor wallet cache: " + cacheErr.Error())
		}
	}
	return err
}

func SharedReclaimExpired() {
	var pending []SharedCharge
	if err := DB.Where("status = ? AND created_at < ?", "reserved", time.Now().Add(-2*time.Hour)).Limit(100).Find(&pending).Error; err != nil {
		return
	}
	for _, p := range pending {
		_ = SharedSettleChannel(p.RequestID, 0)
	}
}

// Native wallet grant retains a unique request id for idempotent admin grants.
func SharedGrant(userID int, quota int64, reference string) error {
	if userID <= 0 || quota <= 0 || quota > 50000000 || reference == "" || len(reference) > 64 {
		return errors.New("invalid grant")
	}
	var credited bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&SharedCharge{RequestID: "grant:" + reference, ConsumerID: userID, OwnerID: userID, Charged: quota, Status: "grant", CreatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		result := tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("user not found")
		}
		credited = true
		return nil
	})
	if err == nil && credited {
		if e := cacheIncrUserQuota(userID, quota); e != nil {
			common.SysLog("shared grant cache: " + e.Error())
		}
	}
	return err
}

// SharedChannelAvailable is consulted by native channel routing so exhausted
// contributor channels are removed from the candidate set before selection.
func SharedChannelAvailable(channelID int) bool {
	var item SharedContribution
	if err := DB.Select("enabled", "cooldown_until", "daily_limit", "daily_used", "reserved", "day", "provider_id").
		Where("channel_id = ?", channelID).First(&item).Error; err != nil {
		return false
	}
	if !item.Enabled || item.CooldownUntil > time.Now().Unix() {
		return false
	}
	var provider SharedProvider
	if err := DB.Select("enabled").First(&provider, item.ProviderID).Error; err != nil || !provider.Enabled {
		return false
	}
	if item.Day != time.Now().UTC().Format("2006-01-02") {
		return true
	}
	return item.DailyLimit-item.DailyUsed-item.Reserved >= 50
}

func SharedSetProviderEnabled(id uint, enabled bool) error {
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&SharedProvider{}).Where("id = ?", id).Update("enabled", enabled)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		var items []SharedContribution
		if err := tx.Where("provider_id = ?", id).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			status := common.ChannelStatusManuallyDisabled
			channelEnabled := enabled && item.Enabled && item.Ciphertext != ""
			if channelEnabled {
				status = common.ChannelStatusEnabled
			}
			if err := tx.Model(&Channel{}).Where("id = ?", item.ChannelID).Update("status", status).Error; err != nil {
				return err
			}
			if err := tx.Model(&Ability{}).Where("channel_id = ?", item.ChannelID).Update("enabled", channelEnabled).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		InitChannelCache()
	}
	return err
}
