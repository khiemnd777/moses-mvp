package service

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/khiemnd777/legal_api/admin/repository"
	"github.com/khiemnd777/legal_api/domain"
)

const (
	defaultTelegramBotTopK = 5
	maxTelegramBotTopK     = 20
	maxWelcomeMessageRunes = 1000
)

var (
	ErrInvalidTelegramBot  = errors.New("invalid telegram bot")
	ErrTelegramBotNotFound = errors.New("telegram bot not found")
	telegramTokenPattern   = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]{20,}$`)
)

type TelegramBotInput struct {
	Name                   string
	Token                  string
	DefaultTone            string
	DefaultTopK            int
	DefaultEffectiveStatus string
	DefaultDomain          string
	DefaultDocType         string
	AllowedChatIDs         []int64
	WelcomeMessage         string
}

type TelegramBotService struct {
	Repo *repository.TelegramBotRepository
}

func NewTelegramBotService(repo *repository.TelegramBotRepository) *TelegramBotService {
	return &TelegramBotService{Repo: repo}
}

func (s *TelegramBotService) List(ctx context.Context) ([]domain.TelegramBot, error) {
	return s.Repo.List(ctx)
}

func (s *TelegramBotService) Get(ctx context.Context, id string) (domain.TelegramBot, error) {
	item, err := s.Repo.GetByID(ctx, id)
	if err == sql.ErrNoRows {
		return domain.TelegramBot{}, ErrTelegramBotNotFound
	}
	return item, err
}

func (s *TelegramBotService) Create(ctx context.Context, input TelegramBotInput) (domain.TelegramBot, error) {
	item, err := normalizeTelegramBotInput(input, true)
	if err != nil {
		return domain.TelegramBot{}, err
	}
	return s.Repo.Create(ctx, item)
}

func (s *TelegramBotService) Update(ctx context.Context, id string, input TelegramBotInput) (domain.TelegramBot, error) {
	item, err := normalizeTelegramBotInput(input, false)
	if err != nil {
		return domain.TelegramBot{}, err
	}
	updateToken := strings.TrimSpace(input.Token) != ""
	updated, err := s.Repo.Update(ctx, id, item, updateToken)
	if err == sql.ErrNoRows {
		return domain.TelegramBot{}, ErrTelegramBotNotFound
	}
	return updated, err
}

func (s *TelegramBotService) Delete(ctx context.Context, id string) error {
	deleted, err := s.Repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrTelegramBotNotFound
	}
	return nil
}

func normalizeTelegramBotInput(input TelegramBotInput, requireToken bool) (domain.TelegramBot, error) {
	name := strings.TrimSpace(input.Name)
	token := strings.TrimSpace(input.Token)
	if name == "" {
		return domain.TelegramBot{}, ErrInvalidTelegramBot
	}
	if requireToken || token != "" {
		if !telegramTokenPattern.MatchString(token) {
			return domain.TelegramBot{}, ErrInvalidTelegramBot
		}
	}
	topK := input.DefaultTopK
	if topK <= 0 {
		topK = defaultTelegramBotTopK
	}
	if topK > maxTelegramBotTopK {
		topK = maxTelegramBotTopK
	}
	tone := strings.TrimSpace(input.DefaultTone)
	if tone == "" {
		tone = "default"
	}
	effectiveStatus := strings.TrimSpace(input.DefaultEffectiveStatus)
	if effectiveStatus == "" {
		effectiveStatus = "active"
	}
	welcomeMessage := strings.TrimSpace(input.WelcomeMessage)
	if utf8.RuneCountInString(welcomeMessage) > maxWelcomeMessageRunes {
		return domain.TelegramBot{}, ErrInvalidTelegramBot
	}
	return domain.TelegramBot{
		Name:                   name,
		Token:                  token,
		TokenHint:              MaskTelegramToken(token),
		DefaultTone:            tone,
		DefaultTopK:            topK,
		DefaultEffectiveStatus: effectiveStatus,
		DefaultDomain:          strings.TrimSpace(input.DefaultDomain),
		DefaultDocType:         strings.TrimSpace(input.DefaultDocType),
		AllowedChatIDs:         dedupeInt64(input.AllowedChatIDs),
		WelcomeMessage:         welcomeMessage,
	}, nil
}

func MaskTelegramToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	prefix, suffix, ok := strings.Cut(token, ":")
	if !ok {
		if len(token) <= 8 {
			return "****"
		}
		return token[:4] + "..." + token[len(token)-4:]
	}
	if len(suffix) <= 4 {
		return prefix + ":****"
	}
	return prefix + ":..." + suffix[len(suffix)-4:]
}

func dedupeInt64(values []int64) []int64 {
	out := make([]int64, 0, len(values))
	seen := map[int64]struct{}{}
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
