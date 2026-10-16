package model

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSharedPoolLedger(t *testing.T) {
	old := DB
	db, err := gorm.Open(sqlite.Open("file:shared-ledger?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() { DB = old; sqlDB.Close() })
	require.NoError(t, SharedMigrate())
	require.NoError(t, DB.Create(&SharedContribution{ID: 1, OwnerID: 10, ProviderID: 5, Ciphertext: "encrypted", Enabled: true, DailyLimit: 50000, Day: time.Now().UTC().Format("2006-01-02")}).Error)
	require.NoError(t, SharedGrant(20, 30000, "one"))
	require.Error(t, SharedGrant(20, 30000, "one"))
	owner, err := SharedReserve(10, 5, 10000, "own")
	require.NoError(t, err)
	require.Equal(t, 10, owner.OwnerID)
	require.NoError(t, SharedSettle("own", 500))
	require.NoError(t, SharedSettle("own", 500))
	var balance SharedAccount
	require.ErrorIs(t, DB.First(&balance, "user_id = ?", 10).Error, gorm.ErrRecordNotFound)
	_, err = SharedReserve(20, 5, 40000, "too-big")
	require.ErrorIs(t, err, ErrSharedInsufficient)
	_, err = SharedReserve(20, 5, 10000, "external")
	require.NoError(t, err)
	balance = SharedAccount{}
	require.NoError(t, DB.First(&balance, "user_id = ?", 20).Error)
	require.EqualValues(t, 20000, balance.Balance)
	require.NoError(t, SharedSettle("external", 1250))
	require.NoError(t, SharedSettle("external", 1250))
	balance = SharedAccount{}
	require.NoError(t, DB.First(&balance, "user_id = ?", 20).Error)
	require.EqualValues(t, 28750, balance.Balance)
	balance = SharedAccount{}
	require.NoError(t, DB.First(&balance, "user_id = ?", 10).Error)
	require.EqualValues(t, 1250, balance.Balance)
	var contributor SharedContribution
	require.NoError(t, DB.First(&contributor, 1).Error)
	require.Zero(t, contributor.Reserved)
	require.EqualValues(t, 1750, contributor.DailyUsed)
	_, err = SharedReserve(20, 5, 1000, "refund")
	require.NoError(t, err)
	require.NoError(t, SharedSettle("refund", 0))
	balance = SharedAccount{}
	require.NoError(t, DB.First(&balance, "user_id = ?", 20).Error)
	require.EqualValues(t, 28750, balance.Balance)
	require.Error(t, SharedSettle("unknown", 0))
	require.True(t, errors.Is(ErrSharedInsufficient, ErrSharedInsufficient))
}
