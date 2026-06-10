package service

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// SubscriptionUsageService tracks successful subscription config downloads
// without making public subscription endpoints depend on analytics writes
// succeeding.
type SubscriptionUsageService struct {
	now func() time.Time
}

// NewSubscriptionUsageService returns a usage tracker using the real clock.
func NewSubscriptionUsageService() *SubscriptionUsageService {
	return &SubscriptionUsageService{now: time.Now}
}

// RecordCompleted increments the durable completed counter after the main
// subscription config has been returned successfully.
func (s *SubscriptionUsageService) RecordCompleted(subscriptionID int, ip, userAgent string) error {
	now := s.clock()
	db := database.GetDB()
	if db == nil {
		return nil
	}

	stats := model.SubscriptionStats{
		SubscriptionId:         subscriptionID,
		CompletedCount:         1,
		LastCompletedAt:        &now,
		LastCompletedIp:        ip,
		LastCompletedUserAgent: userAgent,
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "subscription_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"completed_count":           gorm.Expr("completed_count + ?", 1),
			"last_completed_at":         now,
			"last_completed_ip":         ip,
			"last_completed_user_agent": userAgent,
			"updated_at":                now,
		}),
	}).Create(&stats).Error
}

func (s *SubscriptionUsageService) clock() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
