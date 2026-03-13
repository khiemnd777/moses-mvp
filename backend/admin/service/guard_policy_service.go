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
	ErrInvalidGuardPolicy  = errors.New("invalid guard policy")
	ErrGuardPolicyNotFound = errors.New("guard policy not found")
	validGuardActions      = map[string]struct{}{"refuse": {}, "fallback_llm": {}, "ask_clarification": {}}
)

type GuardPolicyService struct {
	Repo *repository.GuardPolicyRepository
}

func NewGuardPolicyService(repo *repository.GuardPolicyRepository) *GuardPolicyService {
	return &GuardPolicyService{Repo: repo}
}

func (s *GuardPolicyService) List(ctx context.Context) ([]domain.AIGuardPolicy, error) {
	return s.Repo.List(ctx)
}

func (s *GuardPolicyService) Get(ctx context.Context, id string) (domain.AIGuardPolicy, error) {
	item, err := s.Repo.GetByID(ctx, id)
	if err == sql.ErrNoRows {
		return domain.AIGuardPolicy{}, ErrGuardPolicyNotFound
	}
	return item, err
}

func (s *GuardPolicyService) Create(ctx context.Context, item domain.AIGuardPolicy) (domain.AIGuardPolicy, error) {
	if err := validateGuardPolicy(item); err != nil {
		return domain.AIGuardPolicy{}, err
	}
	tx, err := s.Repo.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.AIGuardPolicy{}, err
	}
	defer tx.Rollback()

	if item.Enabled {
		if err := s.Repo.LockSingleEnabledSlot(ctx, tx); err != nil {
			return domain.AIGuardPolicy{}, err
		}
		if err := s.Repo.DisableAllEnabled(ctx, tx); err != nil {
			return domain.AIGuardPolicy{}, err
		}
	}

	created, err := s.Repo.Create(ctx, tx, item)
	if err != nil {
		return domain.AIGuardPolicy{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.AIGuardPolicy{}, err
	}
	return created, nil
}

func (s *GuardPolicyService) Update(ctx context.Context, id string, item domain.AIGuardPolicy) (domain.AIGuardPolicy, error) {
	if err := validateGuardPolicy(item); err != nil {
		return domain.AIGuardPolicy{}, err
	}
	tx, err := s.Repo.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.AIGuardPolicy{}, err
	}
	defer tx.Rollback()

	itemForUpdate := item
	if item.Enabled {
		itemForUpdate.Enabled = false
	}

	updated, err := s.Repo.Update(ctx, tx, id, itemForUpdate)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AIGuardPolicy{}, ErrGuardPolicyNotFound
		}
		return domain.AIGuardPolicy{}, err
	}

	if item.Enabled {
		if err := s.Repo.LockSingleEnabledSlot(ctx, tx); err != nil {
			return domain.AIGuardPolicy{}, err
		}
		if err := s.Repo.DisableOthers(ctx, tx, id); err != nil {
			return domain.AIGuardPolicy{}, err
		}
		updated, err = s.Repo.SetEnabled(ctx, tx, id, true)
		if err != nil {
			if err == sql.ErrNoRows {
				return domain.AIGuardPolicy{}, ErrGuardPolicyNotFound
			}
			return domain.AIGuardPolicy{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.AIGuardPolicy{}, err
	}
	return updated, nil
}

func (s *GuardPolicyService) Delete(ctx context.Context, id string) error {
	deleted, err := s.Repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrGuardPolicyNotFound
	}
	return nil
}

func validateGuardPolicy(item domain.AIGuardPolicy) error {
	if strings.TrimSpace(item.Name) == "" {
		return ErrInvalidGuardPolicy
	}
	if item.MinRetrievedChunks < 0 {
		return ErrInvalidGuardPolicy
	}
	if item.MinSimilarityScore < 0 || item.MinSimilarityScore > 1 {
		return ErrInvalidGuardPolicy
	}
	if _, ok := validGuardActions[item.OnEmptyRetrieval]; !ok {
		return ErrInvalidGuardPolicy
	}
	if _, ok := validGuardActions[item.OnLowConfidence]; !ok {
		return ErrInvalidGuardPolicy
	}
	return nil
}
