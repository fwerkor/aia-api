package model

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Shared credits are micro-USD and never become ordinary wallet quota.
type SharedProvider struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	Name        string  `json:"name"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model" gorm:"uniqueIndex"`
	InputPrice  float64 `json:"input_price"` // USD per million tokens
	OutputPrice float64 `json:"output_price"`
	MaxOutput   int     `json:"max_output"`
	Enabled     bool    `json:"enabled"`
}

type SharedContribution struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	OwnerID       int       `json:"owner_id" gorm:"index"`
	ProviderID    uint      `json:"provider_id" gorm:"index"`
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

type SharedAccount struct {
	UserID  int   `json:"user_id" gorm:"primaryKey"`
	Balance int64 `json:"balance"`
}

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

func SharedMigrate() error {
	return DB.AutoMigrate(&SharedProvider{}, &SharedContribution{}, &SharedAccount{}, &SharedCharge{})
}

var ErrSharedInsufficient = errors.New("shared credits exhausted or no eligible contribution")

func SharedReserve(userID int, providerID uint, amount int64, requestID string) (*SharedContribution, error) {
	if userID <= 0 || amount <= 0 || amount > 100000000 || requestID == "" {
		return nil, fmt.Errorf("invalid reservation")
	}
	var selected SharedContribution
	// Prefer other contributors when credits exist; allow zero-credit self use.
	var balance SharedAccount
	DB.Select("balance").Where("user_id = ?", userID).First(&balance)
	ownRank := "0 ELSE 1"
	if balance.Balance >= amount {
		ownRank = "1 ELSE 0"
	}
	var candidates []SharedContribution
	if err := DB.Where("provider_id = ? AND enabled = ? AND cooldown_until <= ? AND ciphertext <> ?", providerID, true, time.Now().Unix(), "").
		Order("CASE WHEN owner_id = " + fmt.Sprint(userID) + " THEN " + ownRank + " END, daily_used ASC, id").
		Limit(100).Find(&candidates).Error; err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		err := DB.Transaction(func(tx *gorm.DB) error {
			var key SharedContribution
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&key, candidate.ID).Error; err != nil {
				return err
			}
			if !key.Enabled {
				return ErrSharedInsufficient
			}
			today := time.Now().UTC().Format("2006-01-02")
			if key.Day != today {
				key.Day = today
				key.DailyUsed = 0
			}
			if key.DailyLimit <= 0 || key.DailyUsed+key.Reserved+amount > key.DailyLimit {
				return ErrSharedInsufficient
			}
			if key.OwnerID != userID {
				if err := tx.FirstOrCreate(&SharedAccount{}, SharedAccount{UserID: userID}).Error; err != nil {
					return err
				}
				res := tx.Model(&SharedAccount{}).Where("user_id = ? AND balance >= ?", userID, amount).
					Update("balance", gorm.Expr("balance - ?", amount))
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected != 1 {
					return ErrSharedInsufficient
				}
			}
			key.Reserved += amount
			if err := tx.Save(&key).Error; err != nil {
				return err
			}
			if err := tx.Create(&SharedCharge{
				RequestID: requestID, ConsumerID: userID, OwnerID: key.OwnerID, ContributionID: key.ID,
				Reserved: amount, Status: "reserved", CreatedAt: time.Now().UTC(),
			}).Error; err != nil {
				return err
			}
			selected = key
			return nil
		})
		if err == nil {
			return &selected, nil
		}
		if !errors.Is(err, ErrSharedInsufficient) {
			return nil, err
		}
	}
	return nil, ErrSharedInsufficient
}

// SharedSettle is idempotent. Zero actual means the upstream failed and all reserved credits are refunded.
func SharedSettle(requestID string, actual int64) error {
	if actual < 0 {
		return errors.New("negative charge")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var charge SharedCharge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_id = ?", requestID).First(&charge).Error; err != nil {
			return err
		}
		if charge.Status != "reserved" {
			return nil
		}
		if actual > charge.Reserved {
			actual = charge.Reserved
		} // safety bound: never spend outside reservation
		var key SharedContribution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&key, charge.ContributionID).Error; err != nil {
			return err
		}
		key.Reserved -= charge.Reserved
		if key.Reserved < 0 {
			return errors.New("negative reserved quota")
		}
		// Settle to the originating day. Running calls must finish before the next day's reset.
		if key.Day == charge.CreatedAt.UTC().Format("2006-01-02") {
			key.DailyUsed += actual
		}
		if err := tx.Save(&key).Error; err != nil {
			return err
		}
		if charge.ConsumerID != charge.OwnerID {
			if err := tx.FirstOrCreate(&SharedAccount{}, SharedAccount{UserID: charge.ConsumerID}).Error; err != nil {
				return err
			}
			if err := tx.FirstOrCreate(&SharedAccount{}, SharedAccount{UserID: charge.OwnerID}).Error; err != nil {
				return err
			}
			if err := tx.Model(&SharedAccount{}).Where("user_id = ?", charge.ConsumerID).
				Update("balance", gorm.Expr("balance + ?", charge.Reserved-actual)).Error; err != nil {
				return err
			}
			if err := tx.Model(&SharedAccount{}).Where("user_id = ?", charge.OwnerID).
				Update("balance", gorm.Expr("balance + ?", actual)).Error; err != nil {
				return err
			}
		}
		charge.Charged = actual
		charge.Status = "settled"
		return tx.Save(&charge).Error
	})
}

func SharedGrant(userID int, micros int64, reference string) error {
	if userID <= 0 || micros <= 0 || micros > 100000000 || reference == "" {
		return errors.New("invalid grant")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&SharedCharge{RequestID: "grant:" + reference, ConsumerID: userID, OwnerID: userID,
			Charged: micros, Status: "grant", CreatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err := tx.FirstOrCreate(&SharedAccount{}, SharedAccount{UserID: userID}).Error; err != nil {
			return err
		}
		return tx.Model(&SharedAccount{}).Where("user_id = ?", userID).
			Update("balance", gorm.Expr("balance + ?", micros)).Error
	})
}

// Expired reservations can be refunded safely because upstream calls are capped at 60 seconds.
func SharedReclaim() {
	var stale []SharedCharge
	if err := DB.Where("status = ? AND created_at < ?", "reserved", time.Now().Add(-3*time.Minute)).
		Limit(100).Find(&stale).Error; err != nil {
		return
	}
	for _, item := range stale {
		_ = SharedSettle(item.RequestID, 0)
	}
}
