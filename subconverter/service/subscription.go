// Package service implements business logic for the subconverter module.
package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// tokenByteLen is the random byte count behind the public token. Hex encoding
// doubles it, so this produces a 32-character hex string.
const tokenByteLen = 16

const ipCountBatchSize = 500

// Service-layer errors. Controllers translate these into user-facing messages.
var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrInboundsRequired     = errors.New("at least one inbound is required")
	ErrTokenGeneration      = errors.New("token generation failed")
)

// SubscriptionService provides CRUD over subscriptions and their inbound
// junction rows. State is persisted via the package-level database.GetDB().
type SubscriptionService struct{}

// NewSubscriptionService returns a stateless service handle.
func NewSubscriptionService() *SubscriptionService {
	return &SubscriptionService{}
}

// SubscriptionInput is the payload for create/update calls.
//
// Enabled is a pointer so update can distinguish "not set" (keep current) from
// "set to false" (disable). Create treats nil as true (enabled by default).
type SubscriptionInput struct {
	Remark   string         `json:"remark"`
	MaxIps   int            `json:"maxIps"`
	Enabled  *bool          `json:"enabled,omitempty"`
	Inbounds []InboundInput `json:"inbounds"`
}

// SubscriptionDetail is the admin detail projection returned by /get/:id. The
// embedded Subscription keeps the original top-level JSON fields stable while
// adding the management-only detail arrays.
type SubscriptionDetail struct {
	model.Subscription
	BoundIps   []model.IpBinding `json:"boundIps"`
	AccessLogs []model.AccessLog `json:"accessLogs"`
}

// InboundInput is one inbound reference inside a SubscriptionInput.
//
// ClientEmail empty means the subscription will export every client of that
// inbound; non-empty pins it to a single client.
type InboundInput struct {
	InboundId   int    `json:"inboundId"`
	ClientEmail string `json:"clientEmail"`
}

// SubscriptionFormInput is the wire shape the controller binds from the
// panel form (application/x-www-form-urlencoded, matching every other 3X-UI
// API). Form encoding can't faithfully represent nested arrays of structs,
// so the inbound list collapses to a flat `inboundIds` int array; the
// controller widens it back to []InboundInput before calling the service.
type SubscriptionFormInput struct {
	Remark     string `json:"remark" form:"remark"`
	MaxIps     int    `json:"limitIp" form:"limitIp"`
	Enabled    *bool  `json:"enable" form:"enable"`
	InboundIds []int  `json:"inboundIds" form:"inboundIds"`
}

// ToInput widens the flat form into the richer service input shape. Every
// inbound reference is created with an empty ClientEmail (current UI exposes
// inbound-level selection only).
func (f SubscriptionFormInput) ToInput() SubscriptionInput {
	inbounds := make([]InboundInput, 0, len(f.InboundIds))
	for _, id := range f.InboundIds {
		inbounds = append(inbounds, InboundInput{InboundId: id})
	}
	return SubscriptionInput{
		Remark:   f.Remark,
		MaxIps:   f.MaxIps,
		Enabled:  f.Enabled,
		Inbounds: inbounds,
	}
}

// List returns all subscriptions ordered newest-first, with their Inbounds
// junction rows preloaded.
func (s *SubscriptionService) List() ([]model.Subscription, error) {
	var subs []model.Subscription
	db := database.GetDB()
	err := db.
		Preload("Inbounds").
		Preload("Stats").
		Order("id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	if err := attachBoundIPCounts(db, subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// Get returns one subscription with its Inbounds junction rows preloaded.
// Returns ErrSubscriptionNotFound when the row is missing.
func (s *SubscriptionService) Get(id int) (*model.Subscription, error) {
	var sub model.Subscription
	err := database.GetDB().
		Preload("Inbounds").
		Preload("Stats").
		First(&sub, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}
	subs := []model.Subscription{sub}
	if err := attachBoundIPCounts(database.GetDB(), subs); err != nil {
		return nil, err
	}
	sub = subs[0]
	return &sub, nil
}

// GetDetail returns one subscription plus current IP bindings and recent
// access logs for the management details modal.
func (s *SubscriptionService) GetDetail(id int) (*SubscriptionDetail, error) {
	sub, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	boundIps, err := NewIPBindingService().List(id)
	if err != nil {
		return nil, err
	}
	accessLogs, err := NewAccessLogService().List(id)
	if err != nil {
		return nil, err
	}
	if boundIps == nil {
		boundIps = []model.IpBinding{}
	}
	if accessLogs == nil {
		accessLogs = []model.AccessLog{}
	}
	return &SubscriptionDetail{
		Subscription: *sub,
		BoundIps:     boundIps,
		AccessLogs:   accessLogs,
	}, nil
}

// Create writes a subscription plus its inbound rows in one transaction. The
// token is generated server-side and is unique by construction.
func (s *SubscriptionService) Create(input SubscriptionInput) (*model.Subscription, error) {
	if len(input.Inbounds) == 0 {
		return nil, ErrInboundsRequired
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	sub := &model.Subscription{
		Remark:  input.Remark,
		MaxIps:  input.MaxIps,
		Enabled: enabled,
	}

	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		token, err := generateUniqueToken(tx)
		if err != nil {
			return err
		}
		sub.Token = token

		if err := tx.Create(sub).Error; err != nil {
			return err
		}
		if err := insertInbounds(tx, sub.Id, input.Inbounds); err != nil {
			return err
		}
		return tx.Preload("Inbounds").First(sub, sub.Id).Error
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// Update replaces a subscription's mutable fields and its inbound list.
// The token is immutable.
func (s *SubscriptionService) Update(id int, input SubscriptionInput) (*model.Subscription, error) {
	if len(input.Inbounds) == 0 {
		return nil, ErrInboundsRequired
	}

	var sub model.Subscription
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&sub, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSubscriptionNotFound
			}
			return err
		}

		sub.Remark = input.Remark
		sub.MaxIps = input.MaxIps
		if input.Enabled != nil {
			sub.Enabled = *input.Enabled
		}

		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.SubscriptionInbound{}).Error; err != nil {
			return err
		}
		if err := insertInbounds(tx, sub.Id, input.Inbounds); err != nil {
			return err
		}
		return tx.Preload("Inbounds").First(&sub, sub.Id).Error
	})
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// FindByToken loads a subscription by its public token. Returns (nil, nil)
// when the token is unknown so callers can return 404 without inspecting err.
func (s *SubscriptionService) FindByToken(token string) (*model.Subscription, error) {
	var sub model.Subscription
	err := database.GetDB().
		Preload("Inbounds").
		Where("token = ?", token).
		First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// Delete removes the subscription and all rows recorded under it.
func (s *SubscriptionService) Delete(id int) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var sub model.Subscription
		if err := tx.First(&sub, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSubscriptionNotFound
			}
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.SubscriptionInbound{}).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.IpBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.AccessLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.SubscriptionStats{}).Error; err != nil {
			return err
		}
		return tx.Delete(&sub).Error
	})
}

func (s *SubscriptionService) ResetToken(id int) (*model.Subscription, error) {
	var sub model.Subscription
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&sub, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSubscriptionNotFound
			}
			return err
		}
		token, err := generateUniqueToken(tx)
		if err != nil {
			return err
		}
		sub.Token = token
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.IpBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.SubscriptionStats{}).Error; err != nil {
			return err
		}
		if err := tx.Where("subscription_id = ?", id).Delete(&model.AccessLog{}).Error; err != nil {
			return err
		}
		return tx.Preload("Inbounds").Preload("Stats").First(&sub, id).Error
	})
	if err != nil {
		return nil, err
	}
	subs := []model.Subscription{sub}
	if err := attachBoundIPCounts(database.GetDB(), subs); err != nil {
		return nil, err
	}
	sub = subs[0]
	return &sub, nil
}

func insertInbounds(tx *gorm.DB, subID int, items []InboundInput) error {
	for i, in := range items {
		row := model.SubscriptionInbound{
			SubscriptionId: subID,
			InboundId:      in.InboundId,
			ClientEmail:    in.ClientEmail,
			SortOrder:      i,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func attachBoundIPCounts(tx *gorm.DB, subs []model.Subscription) error {
	if len(subs) == 0 {
		return nil
	}

	ids := make([]int, 0, len(subs))
	for _, sub := range subs {
		ids = append(ids, sub.Id)
	}

	countByID := make(map[int]int64, len(subs))
	for start := 0; start < len(ids); start += ipCountBatchSize {
		end := start + ipCountBatchSize
		if end > len(ids) {
			end = len(ids)
		}

		var counts []struct {
			SubscriptionId int
			Count          int64
		}
		if err := tx.Model(&model.IpBinding{}).
			Select("subscription_id, count(*) as count").
			Where("subscription_id IN ?", ids[start:end]).
			Group("subscription_id").
			Scan(&counts).Error; err != nil {
			return err
		}
		for _, row := range counts {
			countByID[row.SubscriptionId] = row.Count
		}
	}
	for i := range subs {
		subs[i].BoundIpCount = countByID[subs[i].Id]
	}
	return nil
}

// generateUniqueToken returns a freshly random token that no existing
// subscription is using. With 16 random bytes the collision probability is
// astronomically low; the retry loop is defensive.
func generateUniqueToken(tx *gorm.DB) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		b := make([]byte, tokenByteLen)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("%w: %v", ErrTokenGeneration, err)
		}
		token := hex.EncodeToString(b)

		var count int64
		if err := tx.Model(&model.Subscription{}).Where("token = ?", token).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return token, nil
		}
	}
	return "", fmt.Errorf("%w: too many collisions", ErrTokenGeneration)
}
