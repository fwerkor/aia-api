package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionPurchaseLimitCountsOnlyCurrentlyActiveSubscriptions(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	user := &User{
		Username: "subscription-limit-user",
		Password: "unused-password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, DB.Create(user).Error)

	plan := &SubscriptionPlan{
		Title:              "Member Basic",
		DurationUnit:       SubscriptionDurationMonth,
		DurationValue:      1,
		TotalAmount:        1000,
		MaxPurchasePerUser: 1,
		Enabled:            true,
	}
	require.NoError(t, DB.Create(plan).Error)

	// A historical subscription whose end time has passed must not block renewal,
	// even if the expiry worker has not yet changed its status field.
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:      user.Id,
		PlanId:      plan.Id,
		AmountTotal: 1000,
		StartTime:   now - 7200,
		EndTime:     now - 1,
		Status:      "active",
	}).Error)

	count, err := CountUserSubscriptionsByPlan(user.Id, plan.Id)
	require.NoError(t, err)
	assert.Zero(t, count)

	first, err := CreateUserSubscriptionFromPlanTx(DB, user.Id, plan, "test")
	require.NoError(t, err)
	require.NotNil(t, first)

	count, err = CountUserSubscriptionsByPlan(user.Id, plan.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	second, err := CreateUserSubscriptionFromPlanTx(DB, user.Id, plan, "test")
	require.Error(t, err)
	assert.Nil(t, second)
	assert.Contains(t, err.Error(), "当前已有有效订阅")

	// Once the current subscription expires, the plan is claimable again.
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", first.Id).Update("end_time", now-1).Error)
	third, err := CreateUserSubscriptionFromPlanTx(DB, user.Id, plan, "test")
	require.NoError(t, err)
	assert.NotNil(t, third)
}
