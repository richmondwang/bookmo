# ADR-003: Rating summaries maintained by database trigger

## Status
Accepted

## Context

Service cards in search results display an average rating. Two implementation options exist:

**Option A — Live query:** Compute `AVG(rating)` from the `reviews` table on every request, joined into the search query.

**Option B — Pre-aggregated summary:** Maintain a `rating_summaries` table with running totals (`total_reviews`, `total_rating`, star-bucket counts) updated incrementally whenever a review is inserted or its status changes.

## Decision

Use Option B — a pre-aggregated `rating_summaries` table, updated by a PostgreSQL trigger on the `reviews` table.

## Reasoning

The search query already does a PostGIS spatial join plus a full-text search rank computation. Adding a correlated `AVG()` subquery for ratings on top of that, for every row in the result set, would add significant query cost at scale. At 100k users with many concurrent search requests, this becomes a meaningful bottleneck.

A trigger-maintained summary table means the rating data is always one indexed row lookup away — `JOIN rating_summaries rs ON rs.service_id = s.id`. No aggregation at query time.

The trigger approach also keeps the aggregation logic in one place (the DB), so it fires correctly regardless of whether writes come from the API, a backfill script, or a migration.

## Consequences

- **Never write to `rating_summaries` directly from application code.** It is owned by the trigger. Application code that writes to it will cause double-counting.
- The trigger stores `total_rating` as a running integer sum, not the average. The average is derived: `avg_rating = ROUND(total_rating::numeric / total_reviews, 1)`. This allows incremental updates without reading existing rows first.
- When a review is removed by moderation (`status` changes to `'removed'`), the trigger subtracts the review's rating from the summary. The application does not need to handle this.
- If `rating_summaries` ever drifts out of sync (detectable via the daily reconciliation job), it can be rebuilt cleanly with:
  ```sql
  TRUNCATE rating_summaries;
  INSERT INTO rating_summaries (service_id, total_reviews, total_rating, avg_rating,
    five_star, four_star, three_star, two_star, one_star)
  SELECT
    service_id,
    COUNT(*),
    SUM(rating),
    ROUND(AVG(rating), 1),
    COUNT(*) FILTER (WHERE rating = 5),
    COUNT(*) FILTER (WHERE rating = 4),
    COUNT(*) FILTER (WHERE rating = 3),
    COUNT(*) FILTER (WHERE rating = 2),
    COUNT(*) FILTER (WHERE rating = 1)
  FROM reviews
  WHERE status = 'published'
  GROUP BY service_id;
  ```
- Tests that insert reviews must account for the trigger firing — the summary row will be created or updated automatically. Do not manually create `rating_summaries` rows in test setup unless testing the summary table itself.
