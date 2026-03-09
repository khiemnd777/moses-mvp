package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/khiemnd777/legal_api/admin/repository"
	"github.com/khiemnd777/legal_api/domain"
)

var (
	ErrInvalidRetrievalConfig  = errors.New("invalid retrieval config")
	ErrRetrievalConfigNotFound = errors.New("retrieval config not found")
)

type RetrievalConfigService struct {
	Repo *repository.RetrievalConfigRepository
}

func NewRetrievalConfigService(repo *repository.RetrievalConfigRepository) *RetrievalConfigService {
	return &RetrievalConfigService{Repo: repo}
}

func (s *RetrievalConfigService) List(ctx context.Context) ([]domain.AIRetrievalConfig, error) {
	return s.Repo.List(ctx)
}

func (s *RetrievalConfigService) Get(ctx context.Context, id string) (domain.AIRetrievalConfig, error) {
	item, err := s.Repo.GetByID(ctx, id)
	if err == sql.ErrNoRows {
		return domain.AIRetrievalConfig{}, ErrRetrievalConfigNotFound
	}
	return item, err
}

func (s *RetrievalConfigService) Create(ctx context.Context, item domain.AIRetrievalConfig) (domain.AIRetrievalConfig, error) {
	if err := validateRetrievalConfig(item); err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	tx, err := s.Repo.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	defer tx.Rollback()

	created, err := s.Repo.Create(ctx, tx, item)
	if err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	if created.Enabled {
		if err := s.Repo.DisableOthers(ctx, tx, created.ID); err != nil {
			return domain.AIRetrievalConfig{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	return created, nil
}

func (s *RetrievalConfigService) Update(ctx context.Context, id string, item domain.AIRetrievalConfig) (domain.AIRetrievalConfig, error) {
	if err := validateRetrievalConfig(item); err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	tx, err := s.Repo.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	defer tx.Rollback()

	updated, err := s.Repo.Update(ctx, tx, id, item)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AIRetrievalConfig{}, ErrRetrievalConfigNotFound
		}
		return domain.AIRetrievalConfig{}, err
	}
	if updated.Enabled {
		if err := s.Repo.DisableOthers(ctx, tx, updated.ID); err != nil {
			return domain.AIRetrievalConfig{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	return updated, nil
}

func (s *RetrievalConfigService) Delete(ctx context.Context, id string) error {
	deleted, err := s.Repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrRetrievalConfigNotFound
	}
	return nil
}

func (s *RetrievalConfigService) Enable(ctx context.Context, id string) (domain.AIRetrievalConfig, error) {
	tx, err := s.Repo.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	defer tx.Rollback()

	updated, err := s.Repo.SetEnabled(ctx, tx, id, true)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AIRetrievalConfig{}, ErrRetrievalConfigNotFound
		}
		return domain.AIRetrievalConfig{}, err
	}
	if err := s.Repo.DisableOthers(ctx, tx, updated.ID); err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AIRetrievalConfig{}, err
	}
	return updated, nil
}

func (s *RetrievalConfigService) Disable(ctx context.Context, id string) (domain.AIRetrievalConfig, error) {
	updated, err := s.Repo.SetEnabled(ctx, s.Repo.Store.DB, id, false)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AIRetrievalConfig{}, ErrRetrievalConfigNotFound
		}
		return domain.AIRetrievalConfig{}, err
	}
	return updated, nil
}

func validateRetrievalConfig(item domain.AIRetrievalConfig) error {
	if strings.TrimSpace(item.Name) == "" {
		return ErrInvalidRetrievalConfig
	}
	if item.DefaultTopK < 1 || item.DefaultTopK > 20 {
		return ErrInvalidRetrievalConfig
	}
	if item.RerankVectorWeight < 0 || item.RerankVectorWeight > 1 {
		return ErrInvalidRetrievalConfig
	}
	if item.RerankKeywordWeight < 0 || item.RerankKeywordWeight > 1 {
		return ErrInvalidRetrievalConfig
	}
	if item.RerankMetadataWeight < 0 || item.RerankMetadataWeight > 1 {
		return ErrInvalidRetrievalConfig
	}
	if item.RerankArticleWeight < 0 || item.RerankArticleWeight > 1 {
		return ErrInvalidRetrievalConfig
	}
	weightSum := item.RerankVectorWeight + item.RerankKeywordWeight + item.RerankMetadataWeight + item.RerankArticleWeight
	if weightSum > 1 {
		return ErrInvalidRetrievalConfig
	}
	if item.AdjacentChunkWindow < 0 || item.AdjacentChunkWindow > 3 {
		return ErrInvalidRetrievalConfig
	}
	if item.MaxContextChunks < 1 || item.MaxContextChunks > 20 {
		return ErrInvalidRetrievalConfig
	}
	if item.MaxContextChars < 1000 || item.MaxContextChars > 200000 {
		return ErrInvalidRetrievalConfig
	}
	if strings.TrimSpace(item.DefaultEffectiveStatus) == "" {
		return ErrInvalidRetrievalConfig
	}
	if item.PreferredDocTypes == nil {
		item.PreferredDocTypes = []string{}
	}
	if item.LegalDomainDefaultsJSON == nil {
		item.LegalDomainDefaultsJSON = map[string]interface{}{}
	}
	return nil
}
