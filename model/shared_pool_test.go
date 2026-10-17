package model

import (
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSharedNativeWalletAndChannel(t *testing.T) {
	old := DB
	db, err := gorm.Open(sqlite.Open("file:shared-native-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() { DB = old; conn.Close() })
	require.NoError(t, db.AutoMigrate(&User{}, &Channel{}, &Ability{}))
	require.NoError(t, SharedMigrate())
	require.NoError(t, db.Create(&User{Id: 10, Quota: 1000, AffCode: "a10", Username: "u10"}).Error)
	require.NoError(t, db.Create(&User{Id: 20, Quota: 30000, AffCode: "a20", Username: "u20"}).Error)
	provider := SharedProvider{Name: "test", BaseURL: "https://example.com/v1", Model: "test-model", Enabled: true, InputPrice: 1, OutputPrice: 3, MaxOutput: 256}
	require.NoError(t, db.Create(&provider).Error)
	m := SharedModel{Code: "test-model", Name: "Test Model", Protocol: "chat", InputPrice: 1, OutputPrice: 3, MaxOutput: 256, Enabled: true}
	require.NoError(t, db.Create(&m).Error)
	require.NoError(t, SharedBindProviderModel(provider.ID, m.ID, true))
	item := SharedContribution{OwnerID: 10, ProviderID: provider.ID, Ciphertext: "encrypted", Fingerprint: "fp-native", Enabled: true, KeyHint: "****test", DailyLimit: 30000, Day: time.Now().UTC().Format("2006-01-02")}
	require.NoError(t, SharedCreateContribution(&item, provider))
	require.NotZero(t, item.ChannelID)
	channel, err := GetChannelById(item.ChannelID, true)
	require.NoError(t, err)
	require.Equal(t, SharedKeyMarker(item.ID), channel.Key)
	require.True(t, SharedIsChannel(channel))
	require.Equal(t, "https://example.com", channel.GetBaseURL())
	var ability Ability
	require.NoError(t, db.First(&ability, "channel_id = ?", item.ChannelID).Error)
	require.True(t, ability.Enabled)
	_, err = SharedReserveChannel(item.ChannelID, 20, "req-too-expensive", 35000)
	require.ErrorIs(t, err, ErrSharedInsufficient)
	found, err := SharedReserveChannel(item.ChannelID, 20, "req-1", 12000)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, SharedSettleChannel("req-1", 1250))
	require.NoError(t, SharedSettleChannel("req-1", 1250))
	var owner, consumer User
	require.NoError(t, db.First(&owner, 10).Error)
	require.NoError(t, db.First(&consumer, 20).Error)
	require.Equal(t, 2250, owner.Quota)
	require.Equal(t, 30000, consumer.Quota) // The native BillingSession, not shared accounting, charges the consumer.
	var after SharedContribution
	require.NoError(t, db.First(&after, item.ID).Error)
	require.EqualValues(t, 1250, after.DailyUsed)
	require.Zero(t, after.Reserved)
	found, err = SharedReserveChannel(item.ChannelID, 20, "req-refund", 5000)
	require.True(t, found)
	require.NoError(t, err)
	require.NoError(t, SharedSettleChannel("req-refund", 0))
	require.NoError(t, db.First(&after, item.ID).Error)
	require.Zero(t, after.Reserved)
	require.NoError(t, SharedGrant(20, 5000, "bootstrap:20"))
	require.Error(t, SharedGrant(20, 5000, "bootstrap:20"))
	consumer = User{}
	require.NoError(t, db.First(&consumer, 20).Error)
	require.Equal(t, 35000, consumer.Quota)
	require.NoError(t, SharedSetEnabled(10, item.ID, false, item.DailyLimit, false))
	_, err = SharedReserveChannel(item.ChannelID, 20, "req-paused", 100)
	require.ErrorIs(t, err, ErrSharedInsufficient)
	require.NoError(t, SharedSetEnabled(10, item.ID, true, item.DailyLimit, false))
	require.NoError(t, SharedSetEnabled(10, item.ID, false, item.DailyLimit, true))
	require.Error(t, SharedSetEnabled(10, item.ID, true, item.DailyLimit, false))
	require.True(t, errors.Is(ErrSharedInsufficient, ErrSharedInsufficient))
}

func TestSharedAPITemplateMigration(t *testing.T) {
	old := DB
	db, err := gorm.Open(sqlite.Open("file:shared-templates?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	raw, _ := db.DB()
	raw.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() { DB = old; raw.Close() })
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	require.NoError(t, SharedMigrate())
	require.NoError(t, SharedMigrate())
	var providers []SharedProvider
	require.NoError(t, db.Order("id").Find(&providers).Error)
	require.Len(t, providers, 4)
	var models []SharedModel
	require.NoError(t, db.Order("id").Find(&models).Error)
	require.GreaterOrEqual(t, len(models), 11)
	var deepseek SharedModel
	require.NoError(t, db.Where("code = ?", "deepseek-flash").First(&deepseek).Error)
	require.Equal(t, "DeepSeek V4.1 Flash", deepseek.Name)
	require.EqualValues(t, 0, deepseek.InputPrice)
	require.True(t, deepseek.Enabled)
	var modelLinks []SharedProviderModel
	require.NoError(t, db.Find(&modelLinks).Error)
	require.Len(t, modelLinks, 4)
	want := map[string]struct {
		protocol, url string
		kind          int
		route         string
		channelURL    string
	}{
		"kimi-k2.5":         {"chat", "https://opencode.ai/zen/v1", constant.ChannelTypeOpenAI, "/v1/chat/completions", "https://opencode.ai/zen"},
		"gpt-6-sol":         {"responses", "https://api.openai.com/v1", constant.ChannelTypeOpenAI, "/v1/responses", "https://api.openai.com"},
		"claude-sonnet-4-6": {"anthropic", "https://api.anthropic.com/v1", constant.ChannelTypeAnthropic, "/v1/chat/completions", "https://api.anthropic.com"},
		"gemini-3.8-flash":  {"gemini", "https://generativelanguage.googleapis.com/v1beta/openai", constant.ChannelTypeGemini, "/v1/chat/completions", "https://generativelanguage.googleapis.com"},
	}
	for idx, p := range providers {
		code := ""
		for modelCode, meta := range want {
			if meta.url == p.BaseURL {
				code = modelCode
				break
			}
		}
		expected, ok := want[code]
		require.True(t, ok)
		require.True(t, p.Enabled)
		require.Equal(t, expected.protocol, p.Protocol)
		require.Equal(t, expected.url, p.BaseURL)
		item := SharedContribution{OwnerID: idx + 1, ProviderID: p.ID, Models: code, Enabled: true,
			Ciphertext: "encrypted", Fingerprint: fmt.Sprintf("test-template-%d", idx), DailyLimit: 10000}
		require.NoError(t, SharedCreateContribution(&item, p))
		channel, err := GetChannelById(item.ChannelID, true)
		require.NoError(t, err)
		require.Equal(t, expected.kind, channel.Type)
		require.Equal(t, expected.channelURL, channel.GetBaseURL())
		require.True(t, SharedChannelSupportsPath(channel.Id, expected.route))
		require.False(t, SharedChannelSupportsPath(channel.Id, "/v1/images/generations"))
	}
}

func TestSharedModelCatalogManyToMany(t *testing.T) {
	old := DB
	db, err := gorm.Open(sqlite.Open("file:shared-many-to-many?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() { DB = old; conn.Close() })
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	require.NoError(t, SharedMigrate())
	var flash, kimi SharedModel
	require.NoError(t, db.Where("code = ?", "deepseek-flash").First(&flash).Error)
	require.NoError(t, db.Where("code = ?", "kimi-k2.5").First(&kimi).Error)
	for idx := 1; idx <= 2; idx++ {
		p := SharedProvider{Name: fmt.Sprintf("Provider-%d", idx), BaseURL: fmt.Sprintf("https://upstream%d.example/v1", idx), Protocol: "chat", Enabled: true}
		require.NoError(t, db.Create(&p).Error)
		require.NoError(t, SharedBindProviderModel(p.ID, flash.ID, true))
		if idx == 1 {
			require.NoError(t, SharedBindProviderModel(p.ID, kimi.ID, true))
			item := SharedContribution{OwnerID: 41, ProviderID: p.ID, Models: "deepseek-flash,kimi-k2.5",
				Enabled: true, DailyLimit: 12000, Ciphertext: "encrypted", Fingerprint: "multi-credential"}
			require.NoError(t, SharedCreateContribution(&item, p))
			channel, err := GetChannelById(item.ChannelID, true)
			require.NoError(t, err)
			require.Equal(t, "deepseek-flash,kimi-k2.5", channel.Models)
			var abilities []Ability
			require.NoError(t, db.Where("channel_id = ?", item.ChannelID).Find(&abilities).Error)
			require.Len(t, abilities, 2)
		}
	}
	list, err := SharedProvidersWithModels(false)
	require.NoError(t, err)
	require.Len(t, list, 6)
	require.Len(t, list[4].Models, 2)
	require.Len(t, list[5].Models, 1)
	require.Error(t, SharedBindProviderModel(list[4].ID, flash.ID, false))
	found, err := SharedModelForProvider(list[5].ID, flash.Code)
	require.NoError(t, err)
	require.Equal(t, flash.Code, found.Code)
}

func TestSharedPendingContributionDoesNotActivateRelay(t *testing.T) {
	old := DB
	db, err := gorm.Open(sqlite.Open("file:shared-admission?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() { DB = old; conn.Close() })
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	require.NoError(t, SharedMigrate())
	var p SharedProvider
	require.NoError(t, db.Where("protocol = ?", "chat").First(&p).Error)
	require.NoError(t, db.Model(&SharedProvider{}).Where("id = ?", p.ID).Update("enabled", false).Error)
	p.Enabled = false
	require.True(t, p.AcceptContributions)
	var selected SharedModel
	require.NoError(t, db.Where("code = ?", "deepseek-flash").First(&selected).Error)
	require.NoError(t, SharedBindVerifiedModels(p.ID, []SharedModel{selected}))
	item := SharedContribution{OwnerID: 456, ProviderID: p.ID, Models: "deepseek-flash",
		DailyLimit: 40000, Ciphertext: "sealed", Fingerprint: "auto-test-only", Enabled: true}
	require.NoError(t, SharedCreateContribution(&item, p))
	channel, err := GetChannelById(item.ChannelID, true)
	require.NoError(t, err)
	require.NotEqual(t, common.ChannelStatusEnabled, channel.Status)
	var ability Ability
	require.NoError(t, db.Where("channel_id = ?", item.ChannelID).First(&ability).Error)
	require.False(t, ability.Enabled)
	require.NoError(t, SharedSetEnabled(item.OwnerID, item.ID, true, item.DailyLimit, false))
	require.NoError(t, db.Where("channel_id = ?", item.ChannelID).First(&ability).Error)
	require.False(t, ability.Enabled)
	require.NoError(t, SharedSetProviderEnabled(p.ID, true))
	require.NoError(t, db.Where("channel_id = ?", item.ChannelID).First(&ability).Error)
	require.True(t, ability.Enabled)
	require.NoError(t, SharedSetProviderEnabled(p.ID, false))
	require.NoError(t, db.Where("channel_id = ?", item.ChannelID).First(&ability).Error)
	require.False(t, ability.Enabled)
}
