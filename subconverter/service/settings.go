package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

const defaultUARejectStatus = http.StatusForbidden

var defaultUAKeywords = []string{"clash", "mihomo", "shadowrocket"}

var ErrUAKeywordsRequired = errors.New("at least one user-agent keyword is required")
var ErrTemplatePreparation = errors.New("selected template could not be prepared")

// SettingsDTO is the API-facing shape of global subconverter settings.
type SettingsDTO struct {
	UAFilterEnabled  bool     `json:"uaFilterEnabled"`
	UAKeywords       []string `json:"uaKeywords"`
	UARejectStatus   int      `json:"uaRejectStatus"`
	TemplateVariant  string   `json:"templateVariant"` // compatibility alias; now a stable template ID
	ActiveTemplateID string   `json:"activeTemplateId"`
}

// SettingsInput is accepted by the settings update endpoint.
type SettingsInput struct {
	UAFilterEnabled bool     `json:"uaFilterEnabled" form:"uaFilterEnabled"`
	UAKeywords      []string `json:"uaKeywords" form:"uaKeywords"`
	UARejectStatus  int      `json:"uaRejectStatus" form:"uaRejectStatus"`
	// Omitted by older clients: preserve the currently selected template.
	TemplateVariant  *string `json:"templateVariant" form:"templateVariant"`
	ActiveTemplateID *string `json:"activeTemplateId" form:"activeTemplateId"`
}

type SettingsService struct{}

func NewSettingsService() *SettingsService {
	return &SettingsService{}
}

func (s *SettingsService) Get() (*SettingsDTO, error) {
	row, err := s.getOrCreate()
	if err != nil {
		return nil, err
	}
	// UA policy is independent of template availability. In-flight readers may
	// still hold an older selection after it has been switched and deleted.
	return settingsToDTO(row), nil
}

func (s *SettingsService) Update(input SettingsInput) (*SettingsDTO, error) {
	if input.ActiveTemplateID != nil {
		if input.TemplateVariant != nil && *input.TemplateVariant != *input.ActiveTemplateID {
			return nil, ErrInvalidTemplateVariant
		}
		input.TemplateVariant = input.ActiveTemplateID
	}
	if input.TemplateVariant != nil {
		if _, err := resolveTemplate(*input.TemplateVariant); err != nil {
			return nil, ErrInvalidTemplateVariant
		}
	}
	keywords := NormalizeUAKeywords(input.UAKeywords)
	if input.UAFilterEnabled && len(keywords) == 0 {
		return nil, ErrUAKeywordsRequired
	}
	if input.UARejectStatus != http.StatusNotFound {
		input.UARejectStatus = defaultUARejectStatus
	}

	// Serialize saves with manual/background refreshes. Readers keep serving
	// the current version while the next version is downloaded and validated.
	templateState.Lock()
	defer templateState.Unlock()
	row, err := s.getOrCreate()
	if err != nil {
		return nil, err
	}
	variant := normalizeTemplateVariant(row.TemplateVariant)
	if input.TemplateVariant != nil {
		variant = *input.TemplateVariant
	}
	if _, err := resolveTemplate(variant); err != nil {
		return nil, ErrInvalidTemplateVariant
	}
	if variant != normalizeTemplateVariant(row.TemplateVariant) {
		// Prepare a usable target cache before committing any settings. Failure
		// keeps both the selected version and the UA settings unchanged.
		if _, err := fetchTemplateVariantLocked(variant); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTemplatePreparation, err)
		}
	}
	row.TemplateVariant = variant
	row.UAFilterEnabled = input.UAFilterEnabled
	row.UAKeywords = strings.Join(keywords, ",")
	row.UARejectStatus = input.UARejectStatus
	if err := database.GetDB().Save(row).Error; err != nil {
		return nil, err
	}
	return settingsToDTO(row), nil
}

func (s *SettingsService) getOrCreate() (*model.Settings, error) {
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("subconverter database is not initialized")
	}
	row := &model.Settings{}
	err := db.First(row, 1).Error
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row = &model.Settings{
		Id:              1,
		UAFilterEnabled: true,
		UAKeywords:      strings.Join(defaultUAKeywords, ","),
		UARejectStatus:  defaultUARejectStatus,
		TemplateVariant: TemplateVariantGeneral,
	}
	if err := db.Create(row).Error; err != nil {
		// Another first request may have created the singleton row concurrently.
		if readErr := db.First(row, 1).Error; readErr == nil {
			return row, nil
		}
		return nil, err
	}
	return row, nil
}

func settingsToDTO(row *model.Settings) *SettingsDTO {
	return &SettingsDTO{
		UAFilterEnabled:  row.UAFilterEnabled,
		UAKeywords:       NormalizeUAKeywords(strings.Split(row.UAKeywords, ",")),
		UARejectStatus:   normalizeUARejectStatus(row.UARejectStatus),
		TemplateVariant:  normalizeTemplateVariant(row.TemplateVariant),
		ActiveTemplateID: normalizeTemplateVariant(row.TemplateVariant),
	}
}

func NormalizeUAKeywords(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}) {
			keyword := strings.ToLower(strings.TrimSpace(part))
			if keyword == "" {
				continue
			}
			if _, ok := seen[keyword]; ok {
				continue
			}
			seen[keyword] = struct{}{}
			out = append(out, keyword)
		}
	}
	return out
}

func IsUserAgentAllowed(ua string, settings *SettingsDTO) bool {
	if settings == nil || !settings.UAFilterEnabled {
		return true
	}
	ua = strings.ToLower(strings.TrimSpace(ua))
	if ua == "" {
		return false
	}
	for _, keyword := range NormalizeUAKeywords(settings.UAKeywords) {
		if strings.Contains(ua, keyword) {
			return true
		}
	}
	return false
}

func normalizeUARejectStatus(status int) int {
	if status == http.StatusNotFound {
		return status
	}
	return defaultUARejectStatus
}
