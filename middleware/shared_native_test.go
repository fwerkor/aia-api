package middleware

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNativeSharedChannelCredentialDecryption(t *testing.T) {
	t.Setenv("AIA_SHARED_SECRET", hex.EncodeToString(make([]byte, 32)))
	old := model.DB
	db, err := gorm.Open(sqlite.Open("file:native-credential?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	model.DB = db
	t.Cleanup(func() { model.DB = old; conn.Close() })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	require.NoError(t, model.SharedMigrate())
	upstream := "sk-test-native-credential"
	block, err := aes.NewCipher(make([]byte, 32))
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, gcm.NonceSize())
	ciphertext := base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(upstream), nil))
	provider := model.SharedProvider{Name: "mock", BaseURL: "https://example.com/v1", Model: "native-test", Enabled: true, MaxOutput: 128}
	require.NoError(t, db.Create(&provider).Error)
	m := model.SharedModel{Code: "native-test", Name: "Native Test", Protocol: "chat", InputPrice: 1, OutputPrice: 3, MaxOutput: 128, Enabled: true}
	require.NoError(t, db.Create(&m).Error)
	require.NoError(t, model.SharedBindProviderModel(provider.ID, m.ID, true))
	item := model.SharedContribution{OwnerID: 5, ProviderID: provider.ID, Enabled: true, Ciphertext: ciphertext, DailyLimit: 50000, Fingerprint: "test-native-fingerprint"}
	require.NoError(t, model.SharedCreateContribution(&item, provider))
	channel, err := model.GetChannelById(item.ChannelID, true)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Nil(t, SetupContextForSelectedChannel(c, channel, provider.Model))
	require.Equal(t, upstream, common.GetContextKeyString(c, constant.ContextKeyChannelKey))
	require.True(t, c.GetBool("aia_shared_channel"))
	normal := &model.Channel{Id: 999, Type: constant.ChannelTypeOpenAI, Key: "sk-normal"}
	require.Nil(t, SetupContextForSelectedChannel(c, normal, "another-model"))
	require.Equal(t, "sk-normal", common.GetContextKeyString(c, constant.ContextKeyChannelKey))
	require.False(t, c.GetBool("aia_shared_channel"))
}
