package controller

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSharedContributionsVisibleBeforeRoutingEnabled(t *testing.T) {
	old := model.DB
	db, err := gorm.Open(sqlite.Open("file:shared-admission-http?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	model.DB = db
	t.Cleanup(func() { model.DB = old; conn.Close() })
	require.NoError(t, model.SharedMigrate())
	gin.SetMode(gin.TestMode)
	run := func() []model.SharedProviderView {
		out := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(out)
		SharedProviders(c)
		require.Equal(t, 200, out.Code)
		var body struct {
			Success bool
			Data    []model.SharedProviderView
		}
		require.NoError(t, json.Unmarshal(out.Body.Bytes(), &body))
		require.True(t, body.Success)
		return body.Data
	}
	items := run()
	require.Len(t, items, 4)
	for _, item := range items {
		require.True(t, item.Enabled)
		require.True(t, item.AcceptContributions)
		require.NotEmpty(t, item.Models)
	}
	require.NoError(t, db.Model(&model.SharedProvider{}).Where("id = ?", items[0].ID).Update("accept_contributions", false).Error)
	require.Len(t, run(), 3)
}
