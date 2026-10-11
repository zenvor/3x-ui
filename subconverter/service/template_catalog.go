package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
	"gorm.io/gorm"
)

const (
	TemplateSourceBuiltin = "builtin"
	TemplateSourceRemote  = "remote"
	TemplateSourceUpload  = "upload" // reserved; no upload endpoint yet
)

var (
	ErrTemplateNotFound = errors.New("template not found")
	ErrTemplateReadOnly = errors.New("built-in templates cannot be edited or deleted")
	ErrTemplateActive   = errors.New("switch the active template before deleting it")
	ErrTemplateInput    = errors.New("invalid template name or URL")
)

type templateDefinition struct {
	ID     string
	Name   string
	Source string
	URL    string
}

func (d templateDefinition) cachePath() string {
	if d.Source == TemplateSourceBuiltin {
		return templateCachePath(d.ID)
	}
	// Bind the cache to both stable identity and source. Changing a URL is
	// prepared in a new file before the DB commits, preserving the active cache.
	hash := sha256.Sum256([]byte(d.URL))
	return filepath.Join(config.GetDBFolderPath(), fmt.Sprintf("mihomo-template-%s-%x.yaml", d.ID, hash))
}

func resolveTemplate(id string) (templateDefinition, error) {
	switch id {
	case TemplateVariantGeneral:
		return templateDefinition{id, "General", TemplateSourceBuiltin, DefaultTemplateURL}, nil
	case TemplateVariantAggressive:
		return templateDefinition{id, "Aggressive", TemplateSourceBuiltin, AggressiveTemplateURL}, nil
	}
	if !isRemoteTemplateID(id) {
		return templateDefinition{}, ErrTemplateNotFound
	}
	db := database.GetDB()
	if db == nil {
		return templateDefinition{}, errors.New("subconverter database is not initialized")
	}
	var row model.Template
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return templateDefinition{}, ErrTemplateNotFound
		}
		return templateDefinition{}, err
	}
	if row.Source != TemplateSourceRemote {
		return templateDefinition{}, ErrTemplateNotFound
	}
	return templateDefinition{row.ID, row.Name, row.Source, row.URL}, nil
}

func isRemoteTemplateID(id string) bool {
	if !strings.HasPrefix(id, "remote-") {
		return false
	}
	_, err := uuid.Parse(strings.TrimPrefix(id, "remote-"))
	return err == nil
}

// Read the selection and its source in one SQLite statement. A switch followed
// by deletion cannot invalidate the definition this request already read.
// Downloads hold templateState, so readers must not acquire that mutex.
func selectedTemplateDefinition() (templateDefinition, error) {
	db := database.GetDB()
	if db == nil {
		return templateDefinition{}, errors.New("subconverter database is not initialized")
	}
	var d templateDefinition
	read := func() error {
		return db.Table("settings").
			Select("settings.template_variant AS id, templates.name, templates.source, templates.url").
			Joins("LEFT JOIN templates ON templates.id = settings.template_variant").
			Where("settings.id = ?", 1).Take(&d).Error
	}
	if err := read(); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return templateDefinition{}, err
		}
		if _, err := NewSettingsService().getOrCreate(); err != nil {
			return templateDefinition{}, err
		}
		if err := read(); err != nil {
			return templateDefinition{}, err
		}
	}
	d.ID = normalizeTemplateVariant(d.ID)
	if d.ID == TemplateVariantGeneral || d.ID == TemplateVariantAggressive {
		return resolveTemplate(d.ID)
	}
	if !isRemoteTemplateID(d.ID) || d.Source != TemplateSourceRemote {
		return templateDefinition{}, ErrTemplateNotFound
	}
	return d, nil
}

type TemplateInput struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// Empty means remote for this version. Explicit upload is rejected.
	Source string `json:"source"`
}

type TemplateRecord struct {
	TemplateStatus
	Builtin bool `json:"builtin"`
}

func ListTemplates() ([]TemplateRecord, error) {
	templateState.Lock()
	defer templateState.Unlock()
	if database.GetDB() == nil {
		return nil, errors.New("subconverter database is not initialized")
	}
	ids := []string{TemplateVariantGeneral, TemplateVariantAggressive}
	var rows []model.Template
	if err := database.GetDB().Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	result := make([]TemplateRecord, 0, len(ids))
	for _, id := range ids {
		d, err := resolveTemplate(id)
		if err != nil {
			return nil, err
		}
		result = append(result, TemplateRecord{templateStatusForDefinition(d), d.Source == TemplateSourceBuiltin})
	}
	return result, nil
}

// SaveTemplate creates or edits a remote source, validating content before any
// source change becomes visible to subscription readers. A rename uses the
// same cache and does not fabricate a successful check.
func SaveTemplate(id string, input TemplateInput) (TemplateRecord, error) {
	input.Name, input.URL = strings.TrimSpace(input.Name), strings.TrimSpace(input.URL)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 || len(input.URL) > 2048 ||
		(input.Source != "" && input.Source != TemplateSourceRemote) {
		return TemplateRecord{}, ErrTemplateInput
	}
	if err := validateRemoteTemplateURL(input.URL); err != nil {
		return TemplateRecord{}, fmt.Errorf("%w: %w", ErrTemplateInput, err)
	}
	templateState.Lock()
	defer templateState.Unlock()
	db := database.GetDB()
	if db == nil {
		return TemplateRecord{}, errors.New("subconverter database is not initialized")
	}
	var old templateDefinition
	if id != "" {
		var err error
		old, err = resolveTemplate(id)
		if err != nil {
			return TemplateRecord{}, err
		}
		if old.Source == TemplateSourceBuiltin {
			return TemplateRecord{}, ErrTemplateReadOnly
		}
	} else {
		id = "remote-" + uuid.NewString()
	}
	d := templateDefinition{id, input.Name, TemplateSourceRemote, input.URL}
	if old.URL != d.URL {
		if _, err := fetchTemplateDefinitionLocked(d); err != nil {
			return TemplateRecord{}, fmt.Errorf("%w: %w", ErrTemplatePreparation, err)
		}
	}
	if old.ID == "" {
		if err := db.Create(&model.Template{ID: id, Name: d.Name, Source: d.Source, URL: d.URL}).Error; err != nil {
			return TemplateRecord{}, err
		}
	} else {
		if err := db.Model(&model.Template{}).Where("id = ?", id).Updates(map[string]any{"name": d.Name, "url": d.URL}).Error; err != nil {
			return TemplateRecord{}, err
		}
	}
	return TemplateRecord{templateStatusForDefinition(d), false}, nil
}

func DeleteTemplate(id string) error {
	templateState.Lock()
	defer templateState.Unlock()
	d, err := resolveTemplate(id)
	if err != nil {
		return err
	}
	if d.Source == TemplateSourceBuiltin {
		return ErrTemplateReadOnly
	}
	active, err := selectedTemplateVariant()
	if err != nil {
		return err
	}
	if id == active {
		return ErrTemplateActive
	}
	// Retain files for in-flight readers that already resolved this definition.
	// UUID identities are never reused, so these cannot leak into another source.
	return database.GetDB().Delete(&model.Template{}, "id = ?", id).Error
}

func RefreshTemplateByID(id string) (TemplateRefreshResult, error) {
	templateState.Lock()
	defer templateState.Unlock()
	d, err := resolveTemplate(id)
	if err != nil {
		return TemplateRefreshResult{}, err
	}
	changed, err := fetchTemplateDefinitionLocked(d)
	return TemplateRefreshResult{TemplateStatus: templateStatusForDefinition(d), Changed: changed}, err
}
