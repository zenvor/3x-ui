package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// ErrIPLimitExceeded means the subscription's MaxIps quota is full and this
// request is from a previously unseen IP.
var ErrIPLimitExceeded = errors.New("ip limit exceeded")

var ErrIPBindingNotFound = errors.New("ip binding not found")

// IPBindingService implements the per-subscription IP-binding rule.
//
// MaxIps semantics:
//   - 0 disables the rule entirely.
//   - >0 caps the number of distinct IPs that may consume the subscription.
type IPBindingService struct{}

// NewIPBindingService returns a stateless service handle.
func NewIPBindingService() *IPBindingService {
	return &IPBindingService{}
}

// Enforce records the IP if it is new (and within quota) or refreshes
// LastSeenAt if it is already bound. Returns ErrIPLimitExceeded when the IP
// is new and the subscription has reached MaxIps.
//
// This is the strict path used by /feed/:token (the canonical entry point).
func (s *IPBindingService) Enforce(subID, maxIps int, ip string) error {
	if ip == "" {
		return errors.New("ip is empty")
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var existing model.IpBinding
		err := tx.Where("subscription_id = ? AND ip = ?", subID, ip).First(&existing).Error
		if err == nil {
			return tx.Model(&existing).Update("last_seen_at", time.Now()).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if maxIps > 0 {
			var count int64
			if err := tx.Model(&model.IpBinding{}).Where("subscription_id = ?", subID).Count(&count).Error; err != nil {
				return err
			}
			if int(count) >= maxIps {
				return ErrIPLimitExceeded
			}
		}

		now := time.Now()
		return tx.Create(&model.IpBinding{
			SubscriptionId: subID,
			Ip:             ip,
			BoundAt:        now,
			LastSeenAt:     now,
		}).Error
	})
}

// CheckOnly is the lenient path used by /feed/:token/nodes (the proxy-provider
// endpoint Mihomo auto-fetches after the main subscription).
//
// It refreshes LastSeenAt for already-bound IPs and allows new IPs through
// only when there is still quota — without registering them. The "no register"
// behaviour matches sublinker: provider hits should never silently consume a
// slot, since the canonical bind happens at /feed/:token.
func (s *IPBindingService) CheckOnly(subID, maxIps int, ip string) error {
	if ip == "" {
		return errors.New("ip is empty")
	}
	if maxIps == 0 {
		return nil
	}
	db := database.GetDB()

	var existing int64
	if err := db.Model(&model.IpBinding{}).
		Where("subscription_id = ? AND ip = ?", subID, ip).
		Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return db.Model(&model.IpBinding{}).
			Where("subscription_id = ? AND ip = ?", subID, ip).
			Update("last_seen_at", time.Now()).Error
	}

	var total int64
	if err := db.Model(&model.IpBinding{}).
		Where("subscription_id = ?", subID).
		Count(&total).Error; err != nil {
		return err
	}
	if int(total) >= maxIps {
		return ErrIPLimitExceeded
	}
	return nil
}

func (s *IPBindingService) List(subscriptionID int) ([]model.IpBinding, error) {
	var rows []model.IpBinding
	if err := database.GetDB().
		Where("subscription_id = ?", subscriptionID).
		Order("last_seen_at desc, id desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *IPBindingService) Delete(subscriptionID, bindingID int) error {
	tx := database.GetDB().
		Where("subscription_id = ?", subscriptionID).
		Delete(&model.IpBinding{}, bindingID)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return ErrIPBindingNotFound
	}
	return nil
}

func (s *IPBindingService) Clear(subscriptionID int) error {
	return database.GetDB().
		Where("subscription_id = ?", subscriptionID).
		Delete(&model.IpBinding{}).Error
}
