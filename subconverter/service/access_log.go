package service

import (
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

const accessLogRetention = 100
const accessLogListMaxLimit = 500
const accessLogUserAgentMaxRunes = 512

const (
	AccessEndpointFull  = "full"
	AccessEndpointNodes = "nodes"

	AccessResultSuccess              = "success"
	AccessResultUARejected           = "ua_rejected"
	AccessResultIPLimitExceeded      = "ip_limit_exceeded"
	AccessResultSubscriptionDisabled = "subscription_disabled"
	AccessResultIPMissing            = "ip_missing"
	AccessResultInternalError        = "internal_error"
)

type AccessLogService struct {
	now func() time.Time
}

type AccessLogInput struct {
	SubscriptionId int
	Endpoint       string
	Ip             string
	UserAgent      string
	StatusCode     int
	Result         string
}

type AccessLogListItem struct {
	model.AccessLog
	SubscriptionRemark string `json:"subscriptionRemark"`
}

func NewAccessLogService() *AccessLogService {
	return &AccessLogService{now: time.Now}
}

func (s *AccessLogService) List(subscriptionID int) ([]model.AccessLog, error) {
	var logs []model.AccessLog
	if err := database.GetDB().
		Where("subscription_id = ?", subscriptionID).
		Order("accessed_at desc, id desc").
		Limit(accessLogRetention).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *AccessLogService) ListRecent(limit int) ([]AccessLogListItem, error) {
	if limit <= 0 || limit > accessLogListMaxLimit {
		limit = accessLogRetention
	}
	var logs []model.AccessLog
	if err := database.GetDB().
		Order("accessed_at desc, id desc").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	if len(logs) == 0 {
		return []AccessLogListItem{}, nil
	}

	subscriptionIDs := make([]int, 0, len(logs))
	seen := make(map[int]struct{}, len(logs))
	for _, log := range logs {
		if _, ok := seen[log.SubscriptionId]; ok {
			continue
		}
		seen[log.SubscriptionId] = struct{}{}
		subscriptionIDs = append(subscriptionIDs, log.SubscriptionId)
	}

	var subs []model.Subscription
	if err := database.GetDB().
		Select("id", "remark").
		Where("id IN ?", subscriptionIDs).
		Find(&subs).Error; err != nil {
		return nil, err
	}
	remarks := make(map[int]string, len(subs))
	for _, sub := range subs {
		remarks[sub.Id] = sub.Remark
	}

	items := make([]AccessLogListItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, AccessLogListItem{
			AccessLog:          log,
			SubscriptionRemark: remarks[log.SubscriptionId],
		})
	}
	return items, nil
}

func (s *AccessLogService) Record(input AccessLogInput) error {
	db := database.GetDB()
	if db == nil || input.SubscriptionId == 0 {
		return nil
	}
	now := s.clock()
	log := model.AccessLog{
		SubscriptionId: input.SubscriptionId,
		Endpoint:       input.Endpoint,
		Ip:             input.Ip,
		UserAgent:      normalizeAccessLogUserAgent(input.UserAgent),
		StatusCode:     input.StatusCode,
		Result:         input.Result,
		AccessedAt:     now,
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		var oldIDs []int
		if err := tx.Model(&model.AccessLog{}).
			Where("subscription_id = ?", input.SubscriptionId).
			Order("accessed_at desc, id desc").
			Offset(accessLogRetention).
			Pluck("id", &oldIDs).Error; err != nil {
			return err
		}
		if len(oldIDs) == 0 {
			return nil
		}
		return tx.Where("id IN ?", oldIDs).Delete(&model.AccessLog{}).Error
	})
}

func normalizeAccessLogUserAgent(userAgent string) string {
	if userAgent == "" {
		return ""
	}
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, userAgent)
	runes := []rune(clean)
	if len(runes) <= accessLogUserAgentMaxRunes {
		return clean
	}
	return string(runes[:accessLogUserAgentMaxRunes])
}

func (s *AccessLogService) clock() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
