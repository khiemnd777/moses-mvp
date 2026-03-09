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
	ErrInvalidPrompt  = errors.New("invalid prompt")
	ErrPromptNotFound = errors.New("prompt not found")
)

type PromptService struct {
	Repo *repository.PromptRepository
}

func NewPromptService(repo *repository.PromptRepository) *PromptService {
	return &PromptService{Repo: repo}
}

func (s *PromptService) List(ctx context.Context) ([]domain.AIPrompt, error) {
	return s.Repo.List(ctx)
}

func (s *PromptService) Get(ctx context.Context, id string) (domain.AIPrompt, error) {
	item, err := s.Repo.GetByID(ctx, id)
	if err == sql.ErrNoRows {
		return domain.AIPrompt{}, ErrPromptNotFound
	}
	return item, err
}

func (s *PromptService) Create(ctx context.Context, item domain.AIPrompt) (domain.AIPrompt, error) {
	if err := validatePrompt(item); err != nil {
		return domain.AIPrompt{}, err
	}
	tx, err := s.Repo.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.AIPrompt{}, err
	}
	defer tx.Rollback()

	created, err := s.Repo.Create(ctx, tx, item)
	if err != nil {
		return domain.AIPrompt{}, err
	}
	if created.Enabled {
		if err := s.Repo.DisableOthersInPromptType(ctx, tx, created.ID, created.PromptType); err != nil {
			return domain.AIPrompt{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.AIPrompt{}, err
	}
	return created, nil
}

func (s *PromptService) Update(ctx context.Context, id string, item domain.AIPrompt) (domain.AIPrompt, error) {
	if err := validatePrompt(item); err != nil {
		return domain.AIPrompt{}, err
	}
	tx, err := s.Repo.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.AIPrompt{}, err
	}
	defer tx.Rollback()

	updated, err := s.Repo.Update(ctx, tx, id, item)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AIPrompt{}, ErrPromptNotFound
		}
		return domain.AIPrompt{}, err
	}
	if updated.Enabled {
		if err := s.Repo.DisableOthersInPromptType(ctx, tx, updated.ID, updated.PromptType); err != nil {
			return domain.AIPrompt{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.AIPrompt{}, err
	}
	return updated, nil
}

func (s *PromptService) Delete(ctx context.Context, id string) error {
	deleted, err := s.Repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrPromptNotFound
	}
	return nil
}

func validatePrompt(item domain.AIPrompt) error {
	if strings.TrimSpace(item.Name) == "" {
		return ErrInvalidPrompt
	}
	if strings.TrimSpace(item.PromptType) == "" {
		return ErrInvalidPrompt
	}
	if strings.TrimSpace(item.SystemPrompt) == "" {
		return ErrInvalidPrompt
	}
	if item.Temperature < 0 || item.Temperature > 1 {
		return ErrInvalidPrompt
	}
	if item.MaxTokens < 1 || item.MaxTokens > 8000 {
		return ErrInvalidPrompt
	}
	if item.Retry < 0 || item.Retry > 5 {
		return ErrInvalidPrompt
	}
	return nil
}
