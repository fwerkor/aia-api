package model

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"
)

type SharedModel struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	Code        string  `json:"code" gorm:"uniqueIndex;not null"`
	Name        string  `json:"-"`
	Protocol    string  `json:"protocol"`
	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
	MaxOutput   int     `json:"max_output"`
	Currency    string  `json:"currency" gorm:"default:USD"`
	Enabled     bool    `json:"enabled"`
}

type SharedProviderModel struct {
	ProviderID uint `json:"provider_id" gorm:"primaryKey"`
	ModelID    uint `json:"model_id" gorm:"primaryKey"`
	Enabled    bool `json:"enabled"`
}

type SharedModelBinding struct {
	SharedModel
	EnabledOnProvider bool `json:"enabled_on_provider"`
}

type SharedProviderView struct {
	SharedProvider
	Models []SharedModelBinding `json:"models"`
}

func SharedCatalogMigrate() error {
	// The legacy table had one model per endpoint, enforcing UNIQUE(model).
	// Remove that index before allowing new independent endpoint records.
	if DB.Migrator().HasIndex(&SharedProvider{}, "idx_shared_providers_model") {
		if err := DB.Migrator().DropIndex(&SharedProvider{}, "idx_shared_providers_model"); err != nil {
			return err
		}
	}
	if err := DB.AutoMigrate(&SharedModel{}, &SharedProviderModel{}); err != nil {
		return err
	}
	seeds := []SharedModel{
		{Code: "deepseek-flash", Name: "DeepSeek V4.1 Flash", Protocol: "chat", MaxOutput: 2048, Enabled: true},
		{Code: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", Protocol: "chat", MaxOutput: 2048, Enabled: true},
		{Code: "gpt-6.1-sol", Name: "GPT-6.1 Sol", Protocol: "responses", InputPrice: 2, OutputPrice: 10, MaxOutput: 2048, Enabled: true},
		{Code: "gpt-6-sol", Name: "GPT-6 Sol", Protocol: "responses", InputPrice: 2, OutputPrice: 10, MaxOutput: 2048, Enabled: true},
		{Code: "gpt-6-luna", Name: "GPT-6 Luna", Protocol: "responses", InputPrice: 0.1, OutputPrice: 0.5, MaxOutput: 2048, Enabled: true},
		{Code: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", Protocol: "anthropic", InputPrice: 3, OutputPrice: 15, MaxOutput: 2048, Enabled: true},
		{Code: "claude-opus-4-8", Name: "Claude Opus 4.8", Protocol: "anthropic", MaxOutput: 2048, Enabled: true},
		{Code: "gemini-3.8-flash", Name: "Gemini 3.8 Flash", Protocol: "gemini", InputPrice: 0.75, OutputPrice: 3.75, MaxOutput: 2048, Enabled: true},
		{Code: "gemini-3.6-flash", Name: "Gemini 3.6 Flash", Protocol: "gemini", MaxOutput: 2048, Enabled: true},
		{Code: "gemini-4-argon", Name: "Gemini 4 Argon", Protocol: "gemini", InputPrice: 2, OutputPrice: 10, MaxOutput: 2048, Enabled: true},
		{Code: "kimi-k2.5", Name: "Kimi K2.5", Protocol: "chat", MaxOutput: 2048, Enabled: true},
	}
	for _, seed := range seeds {
		// Never overwrite administrator catalog edits on restart.
		if err := DB.Where("code = ?", seed.Code).FirstOrCreate(&seed).Error; err != nil {
			return err
		}
	}
	// Endpoint templates and model defaults are independent; configure their
	// initial many-to-many links without putting models into endpoint records.
	defaults := []struct{ baseURL, code string }{
		{"https://opencode.ai/zen/v1", "kimi-k2.5"},
		{"https://opencode.ai/zen/go/v1", "deepseek-flash"},
		{"https://api.openai.com/v1", "gpt-6-sol"},
		{"https://api.anthropic.com/v1", "claude-sonnet-4-6"},
		{"https://generativelanguage.googleapis.com/v1beta/openai", "gemini-3.8-flash"},
	}
	for _, entry := range defaults {
		var provider SharedProvider
		if err := DB.Where("base_url = ?", entry.baseURL).First(&provider).Error; err != nil {
			return err
		}
		var m SharedModel
		if err := DB.Where("code = ?", entry.code).First(&m).Error; err != nil {
			return err
		}
		link := SharedProviderModel{ProviderID: provider.ID, ModelID: m.ID, Enabled: true}
		if err := DB.Where("provider_id = ? AND model_id = ?", provider.ID, m.ID).FirstOrCreate(&link).Error; err != nil {
			return err
		}
	}
	// Set estimated, rounded CNY prices once, preserving subsequent manual edits.
	// Legacy prices were stored as USD per million tokens.
	prices := map[string][2]float64{
		"deepseek-flash": {2, 8}, "deepseek-v4-pro": {9, 28},
		"gpt-6.1-sol": {14, 70}, "gpt-6-sol": {14, 70}, "gpt-6-luna": {1, 4},
		"claude-sonnet-4-6": {21, 105}, "claude-opus-4-8": {35, 175},
		"gemini-3.8-flash": {5, 26}, "gemini-3.6-flash": {3, 15},
		"gemini-4-argon": {14, 70}, "kimi-k2.5": {4, 21},
	}
	for code, price := range prices {
		if err := DB.Model(&SharedModel{}).Where("code = ? AND (currency = ? OR currency IS NULL OR currency = ?)", code, "USD", "").
			Updates(map[string]any{"input_price": price[0], "output_price": price[1], "currency": "CNY", "name": code}).Error; err != nil {
			return err
		}
	}
	// Transfer legacy endpoint/model associations without altering existing channels.
	var providers []SharedProvider
	if err := DB.Find(&providers).Error; err != nil {
		return err
	}
	for _, p := range providers {
		if p.Model == "" {
			continue
		}
		var entry SharedModel
		err := DB.Where("code = ?", p.Model).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			entry = SharedModel{Code: p.Model, Name: p.Model, Protocol: SharedProtocol(p.Protocol), InputPrice: p.InputPrice, OutputPrice: p.OutputPrice, MaxOutput: p.MaxOutput, Enabled: true}
			if err := DB.Create(&entry).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		binding := SharedProviderModel{ProviderID: p.ID, ModelID: entry.ID, Enabled: true}
		if err := DB.Where("provider_id = ? AND model_id = ?", p.ID, entry.ID).FirstOrCreate(&binding).Error; err != nil {
			return err
		}
	}
	return nil
}

func SharedProviderModels(providerID uint, onlyActive bool) ([]SharedModelBinding, error) {
	var bindings []SharedProviderModel
	query := DB.Where("provider_id = ?", providerID)
	if onlyActive {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Find(&bindings).Error; err != nil {
		return nil, err
	}
	result := make([]SharedModelBinding, 0, len(bindings))
	for _, binding := range bindings {
		var m SharedModel
		if err := DB.First(&m, binding.ModelID).Error; err != nil {
			return nil, err
		}
		if onlyActive && !m.Enabled {
			continue
		}
		result = append(result, SharedModelBinding{SharedModel: m, EnabledOnProvider: binding.Enabled})
	}
	return result, nil
}

func SharedModelCandidates(providerID uint, protocol string) ([]SharedModel, error) {
	var entries []SharedModel
	if err := DB.Where("protocol = ? AND enabled = ?", SharedProtocol(protocol), true).
		Order("id").Limit(8).Find(&entries).Error; err != nil {
		return nil, err
	}
	var bindings []SharedProviderModel
	if err := DB.Where("provider_id = ?", providerID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	excluded := map[uint]bool{}
	for _, b := range bindings {
		if !b.Enabled {
			excluded[b.ModelID] = true
		}
	}
	result := make([]SharedModel, 0, len(entries))
	for _, m := range entries {
		if !excluded[m.ID] {
			result = append(result, m)
		}
	}
	return result, nil
}

func SharedProvidersWithModels(onlyActive bool) ([]SharedProviderView, error) {
	var providers []SharedProvider
	query := DB.Order("id")
	if onlyActive {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Find(&providers).Error; err != nil {
		return nil, err
	}
	result := make([]SharedProviderView, 0, len(providers))
	for _, p := range providers {
		bindings, err := SharedProviderModels(p.ID, onlyActive)
		if err != nil {
			return nil, err
		}
		result = append(result, SharedProviderView{SharedProvider: p, Models: bindings})
	}
	return result, nil
}

func SharedModelForProvider(providerID uint, code string) (*SharedModel, error) {
	var m SharedModel
	err := DB.Table("shared_models AS m").
		Select("m.*").
		Joins("JOIN shared_provider_models AS pm ON pm.model_id = m.id").
		Where("pm.provider_id = ? AND pm.enabled = ? AND m.enabled = ? AND m.code = ?", providerID, true, true, code).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func SharedVerifyModels(providerID uint, codes []string) ([]SharedModel, error) {
	if len(codes) == 0 || len(codes) > 12 {
		return nil, errors.New("choose between 1 and 12 models")
	}
	models := make([]SharedModel, 0, len(codes))
	seen := make(map[string]bool)
	for _, code := range codes {
		if code == "" || strings.ContainsAny(code, ", \t\n") || seen[code] {
			return nil, fmt.Errorf("duplicate or invalid model: %s", code)
		}
		seen[code] = true
		m, err := SharedModelForProvider(providerID, code)
		if err != nil {
			return nil, fmt.Errorf("model %s is not enabled for this endpoint", code)
		}
		models = append(models, *m)
	}
	return models, nil
}

func SharedBindProviderModel(providerID, modelID uint, enabled bool) error {
	var p SharedProvider
	if err := DB.First(&p, providerID).Error; err != nil {
		return err
	}
	var m SharedModel
	if err := DB.First(&m, modelID).Error; err != nil {
		return err
	}
	if SharedProtocol(p.Protocol) != SharedProtocol(m.Protocol) {
		return errors.New("model protocol differs from endpoint protocol")
	}
	var count int64
	if err := DB.Model(&SharedContribution{}).Where("provider_id = ?", providerID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("model mapping cannot change while contributed keys exist")
	}
	row := SharedProviderModel{ProviderID: providerID, ModelID: modelID, Enabled: enabled}
	return DB.Save(&row).Error
}

// SharedBindVerifiedModels only records pairs for which a submitted API key
// successfully completed a real low-output completion call. This operation
// cannot disable existing mappings or modify any existing contributor channel.
func SharedBindVerifiedModels(providerID uint, selected []SharedModel) error {
	if len(selected) == 0 || len(selected) > 12 {
		return errors.New("no verified models")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var p SharedProvider
		if err := tx.First(&p, providerID).Error; err != nil {
			return err
		}
		for _, m := range selected {
			var current SharedModel
			if err := tx.First(&current, m.ID).Error; err != nil {
				return err
			}
			if !current.Enabled || SharedProtocol(p.Protocol) != SharedProtocol(current.Protocol) {
				return errors.New("verified model is not enabled for endpoint protocol")
			}
			link := SharedProviderModel{ProviderID: providerID, ModelID: current.ID, Enabled: true}
			if err := tx.Where("provider_id = ? AND model_id = ?", providerID, current.ID).FirstOrCreate(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
