package search

import (
	"context"
	"fmt"
)

const (
	defaultRadius = 5000.0
	defaultLimit  = 20
	maxLimit      = 50
)

// Service contains the business logic for the search module.
type Service struct {
	repo  *Repository
	cache *Cache
}

// NewService constructs a Service.
func NewService(repo *Repository, cache *Cache) *Service {
	return &Service{repo: repo, cache: cache}
}

// Search validates parameters and executes a browse or keyword search.
// Browse mode uses the Redis feed cache; search mode always hits the DB.
func (s *Service) Search(ctx context.Context, p SearchParams) (*SearchResponse, error) {
	if p.Lat < -90 || p.Lat > 90 || p.Lng < -180 || p.Lng > 180 {
		return nil, ErrInvalidCoordinates
	}

	// Apply defaults.
	if p.Radius <= 0 {
		p.Radius = defaultRadius
	}
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	if p.Limit > maxLimit {
		p.Limit = maxLimit
	}

	// Browse mode: attempt cache lookup.
	if p.Query == "" {
		cached, found, err := s.cache.GetFeed(ctx, p.Lat, p.Lng, p.CategorySlug)
		if err != nil {
			// Cache errors are non-fatal; fall through to DB.
			_ = err
		}
		if found {
			// Apply pagination to cached results.
			total := len(cached)
			start := p.Offset
			if start > total {
				start = total
			}
			end := start + p.Limit
			if end > total {
				end = total
			}
			return &SearchResponse{Results: cached[start:end], Total: total}, nil
		}

		// Cache miss: query DB.
		results, err := s.repo.Search(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("search.Service.Search: %w", err)
		}
		if results == nil {
			results = []ServiceResult{}
		}

		// Cache the full (unpaginated) result for offset=0 requests only,
		// so the cache entry represents the complete ranked list for this cell.
		if p.Offset == 0 {
			_ = s.cache.SetFeed(ctx, p.Lat, p.Lng, p.CategorySlug, results)
		}

		return &SearchResponse{Results: results, Total: len(results)}, nil
	}

	// Search mode: always hit the DB.
	results, err := s.repo.Search(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("search.Service.Search: %w", err)
	}
	if results == nil {
		results = []ServiceResult{}
	}
	return &SearchResponse{Results: results, Total: len(results)}, nil
}

// GetCategories returns all categories.
func (s *Service) GetCategories(ctx context.Context) ([]Category, error) {
	cats, err := s.repo.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("search.Service.GetCategories: %w", err)
	}
	if cats == nil {
		cats = []Category{}
	}
	return cats, nil
}

// GetSuggestions returns autocomplete suggestions for a search prefix.
// MVP: returns an empty slice; a real implementation would use Redis ZRANGEBYLEX.
func (s *Service) GetSuggestions(_ context.Context, _ string) ([]string, error) {
	return []string{}, nil
}

// IncrementSuggestionScore records a search term for future autocomplete ranking.
// MVP: no-op.
func (s *Service) IncrementSuggestionScore(_ context.Context, _ string) error {
	return nil
}
