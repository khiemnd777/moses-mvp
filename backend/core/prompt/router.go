package prompt

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/khiemnd777/legal_api/domain"
)

const DefaultPromptType = "legal_guard"

type Store interface {
	ListEnabledAIPrompts(ctx context.Context) ([]domain.AIPrompt, error)
}

type Router struct {
	store       Store
	defaultType string
	ttl         time.Duration

	mu       sync.RWMutex
	cache    map[string]domain.AIPrompt
	loadedAt time.Time
	ready    bool
}

func NewRouter(store Store, ttl time.Duration, defaultType string) *Router {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if strings.TrimSpace(defaultType) == "" {
		defaultType = DefaultPromptType
	}
	return &Router{
		store:       store,
		defaultType: strings.TrimSpace(defaultType),
		ttl:         ttl,
		cache:       map[string]domain.AIPrompt{},
	}
}

func (r *Router) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = false
	r.loadedAt = time.Time{}
}

func (r *Router) GetPrompt(ctx context.Context, promptType string) (domain.AIPrompt, string, error) {
	cache, err := r.ensureCache(ctx)
	if err != nil {
		return domain.AIPrompt{}, "", err
	}

	requested := strings.TrimSpace(promptType)
	if requested != "" {
		if prompt, ok := cache[requested]; ok {
			return prompt, requested, nil
		}
	}

	if fallback, ok := cache[r.defaultType]; ok {
		return fallback, r.defaultType, nil
	}
	return domain.AIPrompt{}, "", fmt.Errorf("missing prompt for type=%q and default_type=%q", requested, r.defaultType)
}

func (r *Router) ensureCache(ctx context.Context) (map[string]domain.AIPrompt, error) {
	r.mu.RLock()
	if r.ready && time.Since(r.loadedAt) <= r.ttl {
		out := clonePromptMap(r.cache)
		r.mu.RUnlock()
		return out, nil
	}
	r.mu.RUnlock()

	rows, err := r.store.ListEnabledAIPrompts(ctx)
	if err != nil {
		return nil, err
	}
	next := make(map[string]domain.AIPrompt, len(rows))
	for _, p := range rows {
		pt := strings.TrimSpace(p.PromptType)
		if pt == "" {
			continue
		}
		current, ok := next[pt]
		if !ok || p.UpdatedAt.After(current.UpdatedAt) {
			next[pt] = p
		}
	}

	r.mu.Lock()
	r.cache = next
	r.loadedAt = time.Now()
	r.ready = true
	out := clonePromptMap(r.cache)
	r.mu.Unlock()
	return out, nil
}

func clonePromptMap(in map[string]domain.AIPrompt) map[string]domain.AIPrompt {
	out := make(map[string]domain.AIPrompt, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
