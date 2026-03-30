package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the search module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Search executes a browse or keyword search query and returns matching services.
//
// Browse mode (p.Query == ""): sorts by distance * 0.6 - avg_rating * 0.4.
// Search mode (p.Query != ""): filters by FTS and sorts by relevance + avg_rating * 0.2.
func (r *Repository) Search(ctx context.Context, p SearchParams) ([]ServiceResult, error) {
	// Common SELECT and FROM/JOIN clause. Parameter $1 = lat, $2 = lng.
	const selectClause = `
SELECT
    s.id,
    s.name,
    s.description,
    s.price_per_unit,
    s.min_duration,
    s.max_duration,
    s.capacity_type,
    s.tags,
    COALESCE(s.allows_participants, c.allows_participants, cp.allows_participants, false) AS allows_participants,
    b.id           AS branch_id,
    b.name         AS branch_name,
    b.address      AS branch_address,
    ST_Distance(b.location::geography, ST_MakePoint($2, $1)::geography) AS distance_meters,
    c.id::text     AS category_id,
    c.name         AS category_name,
    COALESCE(rs.avg_rating, 0)     AS avg_rating,
    COALESCE(rs.total_reviews, 0)  AS total_reviews`

	const fromClause = `
FROM services s
JOIN branches b      ON b.id = s.branch_id AND b.deleted_at IS NULL
LEFT JOIN categories c  ON c.id = s.category_id
LEFT JOIN categories cp ON cp.id = c.parent_id
LEFT JOIN rating_summaries rs ON rs.service_id = s.id`

	// Build WHERE conditions and args. $1=lat, $2=lng are always present.
	// $3 = radius, and additional params are appended as needed.
	args := []any{p.Lat, p.Lng, p.Radius}
	argIdx := 4 // next placeholder index

	var whereParts []string
	whereParts = append(whereParts, "s.deleted_at IS NULL")
	whereParts = append(whereParts, fmt.Sprintf("ST_DWithin(b.location::geography, ST_MakePoint($2, $1)::geography, $3)"))

	var orderClause string
	var rankSelect string

	if p.Query != "" {
		// Search mode: FTS filter + rank by relevance + rating.
		args = append(args, p.Query)
		rankSelect = fmt.Sprintf(", ts_rank(s.search_vec, plainto_tsquery('english', $%d)) AS rank", argIdx)
		whereParts = append(whereParts, fmt.Sprintf("s.search_vec @@ plainto_tsquery('english', $%d)", argIdx))
		argIdx++
		orderClause = "ORDER BY rank + COALESCE(rs.avg_rating, 0) * 0.2 DESC"
	} else {
		// Browse mode: sort by distance * 0.6 - rating * 0.4.
		rankSelect = ""
		orderClause = "ORDER BY distance_meters * 0.6 - COALESCE(rs.avg_rating, 0) * 0.4"
	}

	if p.CategorySlug != "" {
		args = append(args, p.CategorySlug)
		whereParts = append(whereParts, fmt.Sprintf("c.slug = $%d", argIdx))
		argIdx++
	}

	// LIMIT and OFFSET.
	args = append(args, p.Limit, p.Offset)
	limitClause := fmt.Sprintf("LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	whereClause := "WHERE " + strings.Join(whereParts, "\n  AND ")

	q := selectClause + rankSelect + "\n" + fromClause + "\n" + whereClause + "\n" + orderClause + "\n" + limitClause

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search.Repository.Search: %w", err)
	}
	defer rows.Close()

	var results []ServiceResult
	for rows.Next() {
		var sr ServiceResult
		var tags []string

		var scanArgs []any
		if p.Query != "" {
			var rank float64
			scanArgs = []any{
				&sr.ID, &sr.Name, &sr.Description,
				&sr.PricePerUnit, &sr.MinDuration, &sr.MaxDuration,
				&sr.CapacityType, &tags, &sr.AllowsParticipants,
				&sr.BranchID, &sr.BranchName, &sr.BranchAddress,
				&sr.DistanceMeters,
				&sr.CategoryID, &sr.CategoryName,
				&sr.AvgRating, &sr.TotalReviews,
				&rank,
			}
		} else {
			scanArgs = []any{
				&sr.ID, &sr.Name, &sr.Description,
				&sr.PricePerUnit, &sr.MinDuration, &sr.MaxDuration,
				&sr.CapacityType, &tags, &sr.AllowsParticipants,
				&sr.BranchID, &sr.BranchName, &sr.BranchAddress,
				&sr.DistanceMeters,
				&sr.CategoryID, &sr.CategoryName,
				&sr.AvgRating, &sr.TotalReviews,
			}
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("search.Repository.Search scan: %w", err)
		}
		sr.Tags = tags
		results = append(results, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search.Repository.Search rows: %w", err)
	}
	return results, nil
}

// GetCategories returns all categories ordered by name.
func (r *Repository) GetCategories(ctx context.Context) ([]Category, error) {
	const q = `
SELECT
    id::text,
    name,
    slug,
    icon_url,
    parent_id::text
FROM categories
ORDER BY name`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search.Repository.GetCategories: %w", err)
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.IconURL, &cat.ParentID); err != nil {
			return nil, fmt.Errorf("search.Repository.GetCategories scan: %w", err)
		}
		cats = append(cats, cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search.Repository.GetCategories rows: %w", err)
	}
	return cats, nil
}
