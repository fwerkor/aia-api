package model

import (
	"errors"
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
	item := SharedContribution{OwnerID: 10, ProviderID: provider.ID, Ciphertext: "encrypted", Fingerprint: "fp-native", Enabled: true, KeyHint: "****test", DailyLimit: 30000, Day: time.Now().UTC().Format("2006-01-02")}
	require.NoError(t, SharedCreateContribution(&item, provider))
	require.NotZero(t, item.ChannelID)
	channel, err := GetChannelById(item.ChannelID, true)
	require.NoError(t, err)
	require.Equal(t, SharedKeyMarker(item.ID), channel.Key)
	require.True(t, SharedIsChannel(channel))
	require.Equal(t, provider.BaseURL, channel.GetBaseURL())
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
