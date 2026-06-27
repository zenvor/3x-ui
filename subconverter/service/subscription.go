// Package service implements business logic for the subconverter module.
package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"

	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xservice "github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// tokenByteLen is the random byte count behind the public token. Hex encoding
// doubles it, so this produces a 32-character hex string.
const tokenByteLen = 16

const ipCountBatchSize = 500

// Service-layer errors. Controllers translate these into user-facing messages.
var (
	ErrSubscriptionNotFound  = errors.New("subscription not found")
	ErrInboundsRequired      = errors.New("at least one inbound is required")
	ErrTokenGeneration       = errors.New("token generation failed")
	ErrCDNServerRequired     = errors.New("cdn server is required when CDN TLS override is enabled")
	ErrCommonClientRequired  = errors.New("selected inbounds do not share an enabled exportable client")
	ErrCommonClientAmbiguous = errors.New("multiple common clients found; select one client")
	ErrSelectedClientInvalid = errors.New("selected client is not available on every inbound")
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
	Remark       string         `json:"remark"`
	MaxIps       int            `json:"maxIps"`
	Enabled      *bool          `json:"enabled,omitempty"`
	TrafficStats bool           `json:"trafficStats"`
	Inbounds     []InboundInput `json:"inbounds"`
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
	InboundId     int    `json:"inboundId"`
	ClientEmail   string `json:"clientEmail"`
	CdnTLS        bool   `json:"cdnTls"`
	CdnServer     string `json:"cdnServer"`
	CdnPort       int    `json:"cdnPort"`
	CdnServerName string `json:"cdnServerName"`
	CdnClientFp   string `json:"cdnClientFingerprint"`
}

// SubscriptionFormInput is the admin API wire shape. Newer callers send the
// richer JSON `inbounds` list; `inboundIds` remains for older flat payloads.
type SubscriptionFormInput struct {
	Remark       string         `json:"remark" form:"remark"`
	MaxIps       int            `json:"limitIp" form:"limitIp"`
	Enabled      *bool          `json:"enable" form:"enable"`
	TrafficStats bool           `json:"trafficStats" form:"trafficStats"`
	InboundIds   []int          `json:"inboundIds" form:"inboundIds"`
	Inbounds     []InboundInput `json:"inbounds" form:"inbounds"`
}

// ToInput widens the flat form into the richer service input shape.
func (f SubscriptionFormInput) ToInput() SubscriptionInput {
	inbounds := f.Inbounds
	if len(inbounds) == 0 {
		inbounds = make([]InboundInput, 0, len(f.InboundIds))
		for _, id := range f.InboundIds {
			inbounds = append(inbounds, InboundInput{InboundId: id})
		}
	}
	return SubscriptionInput{
		Remark:       f.Remark,
		MaxIps:       f.MaxIps,
		Enabled:      f.Enabled,
		TrafficStats: f.TrafficStats,
		Inbounds:     inbounds,
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
	if err := prepareInboundInputs(input.Inbounds); err != nil {
		return nil, err
	}
	if input.TrafficStats {
		if err := s.bindCommonClient(&input); err != nil {
			return nil, err
		}
	} else {
		clearInboundClientEmails(input.Inbounds)
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	sub := &model.Subscription{
		Remark:       input.Remark,
		MaxIps:       input.MaxIps,
		Enabled:      enabled,
		TrafficStats: input.TrafficStats,
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
	if err := prepareInboundInputs(input.Inbounds); err != nil {
		return nil, err
	}
	if input.TrafficStats && shouldBindTrafficStats(input) {
		if err := s.bindCommonClient(&input); err != nil {
			return nil, err
		}
	} else {
		if !input.TrafficStats {
			clearInboundClientEmails(input.Inbounds)
		}
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
		sub.TrafficStats = input.TrafficStats
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
	for i := range items {
		in := items[i]
		if in.CdnTLS && in.CdnServer == "" {
			return ErrCDNServerRequired
		}
		row := model.SubscriptionInbound{
			SubscriptionId: subID,
			InboundId:      in.InboundId,
			ClientEmail:    in.ClientEmail,
			SortOrder:      i,
			CdnTLS:         in.CdnTLS,
			CdnServer:      in.CdnServer,
			CdnPort:        in.CdnPort,
			CdnServerName:  in.CdnServerName,
			CdnClientFp:    in.CdnClientFp,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeInboundInput(in *InboundInput) {
	in.ClientEmail = strings.TrimSpace(in.ClientEmail)
	in.CdnServer = strings.TrimSpace(in.CdnServer)
	in.CdnServerName = strings.TrimSpace(in.CdnServerName)
	in.CdnClientFp = strings.TrimSpace(in.CdnClientFp)
	if !in.CdnTLS {
		in.CdnServer = ""
		in.CdnPort = 0
		in.CdnServerName = ""
		in.CdnClientFp = ""
		return
	}
	if in.CdnPort == 0 {
		in.CdnPort = 443
	}
	if in.CdnServerName == "" {
		in.CdnServerName = in.CdnServer
	}
	if in.CdnClientFp == "" {
		in.CdnClientFp = "chrome"
	}
}

func prepareInboundInputs(items []InboundInput) error {
	for i := range items {
		normalizeInboundInput(&items[i])
		if items[i].CdnTLS && items[i].CdnServer == "" {
			return ErrCDNServerRequired
		}
	}
	return nil
}

func clearInboundClientEmails(items []InboundInput) {
	for i := range items {
		items[i].ClientEmail = ""
	}
}

func shouldBindTrafficStats(input SubscriptionInput) bool {
	return input.Enabled == nil || *input.Enabled
}

func (s *SubscriptionService) bindCommonClient(input *SubscriptionInput) error {
	for i := range input.Inbounds {
		normalizeInboundInput(&input.Inbounds[i])
	}
	email, err := resolveCommonClientEmail(input.Inbounds, &xservice.InboundService{})
	if err != nil {
		return err
	}
	for i := range input.Inbounds {
		input.Inbounds[i].ClientEmail = email
	}
	return nil
}

func resolveCommonClientEmail(items []InboundInput, inboundSvc *xservice.InboundService) (string, error) {
	if len(items) == 0 {
		return "", ErrInboundsRequired
	}

	explicitEmail := ""
	for _, item := range items {
		if item.ClientEmail == "" {
			continue
		}
		if explicitEmail == "" {
			explicitEmail = item.ClientEmail
			continue
		}
		if item.ClientEmail != explicitEmail {
			return "", ErrSelectedClientInvalid
		}
	}

	var common []string
	for i, item := range items {
		inbound, err := inboundSvc.GetInbound(item.InboundId)
		if err != nil || inbound == nil {
			return "", ErrCommonClientRequired
		}
		if !IsVlessSupportedInbound(inbound, proxyOptionsFromInput(item)) {
			return "", ErrCommonClientRequired
		}
		clients, err := inboundSvc.GetClients(inbound)
		if err != nil {
			return "", err
		}
		emails := exportableClientEmails(clients)
		if len(emails) == 0 {
			return "", ErrCommonClientRequired
		}
		if explicitEmail != "" {
			if !slices.Contains(emails, explicitEmail) {
				return "", ErrSelectedClientInvalid
			}
			continue
		}
		if i == 0 {
			common = emails
			continue
		}
		common = intersectEmails(common, emails)
		if len(common) == 0 {
			return "", ErrCommonClientRequired
		}
	}

	if explicitEmail != "" {
		return explicitEmail, nil
	}
	if len(common) == 1 {
		return common[0], nil
	}
	return "", ErrCommonClientAmbiguous
}

func exportableClientEmails(clients []xmodel.Client) []string {
	emails := make([]string, 0, len(clients))
	seen := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		if !isExportableClient(client) {
			continue
		}
		email := strings.TrimSpace(client.Email)
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		emails = append(emails, email)
	}
	return emails
}

func intersectEmails(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, email := range right {
		rightSet[email] = struct{}{}
	}
	out := make([]string, 0, min(len(left), len(right)))
	for _, email := range left {
		if _, ok := rightSet[email]; ok {
			out = append(out, email)
		}
	}
	return out
}

func proxyOptionsFromInput(in InboundInput) ProxyOptions {
	if !in.CdnTLS {
		return ProxyOptions{}
	}
	return ProxyOptions{CDNTLS: &CDNTLSOptions{
		Enabled:    in.CdnTLS,
		Server:     in.CdnServer,
		Port:       in.CdnPort,
		Servername: in.CdnServerName,
		ClientFp:   in.CdnClientFp,
	}}
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
