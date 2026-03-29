# ADR-004: Geohash precision-5 grid cells for home feed caching

## Status
Accepted

## Context

The home screen loads a ranked list of nearby services for the customer's current location. Running a full PostGIS + FTS query on every home screen open is acceptable at low scale but becomes expensive with many concurrent users — especially since most users in the same neighborhood will receive near-identical results.

The options considered:

**Option A — No cache:** Run the PostGIS query on every request. Simple, always fresh, but expensive at scale.

**Option B — Cache per exact coordinates:** Use the customer's GPS coordinates as a cache key. Near-zero cache hit rate since no two users share the exact same GPS point.

**Option C — Cache per grid cell:** Divide the map into fixed grid cells. All customers within the same cell share a cached feed. Trade freshness for efficiency.

## Decision

Use Geohash precision-5 grid cells as cache keys in Redis. Key format: `feed:{geohash5}:{category_slug}`, e.g. `feed:wdw2q:all`.

Geohash precision-5 produces cells of approximately 4.9km × 4.9km — large enough to get meaningful cache hits in dense urban areas (Metro Manila, Cebu, Davao) but small enough that the results remain locally relevant.

TTL: 5 minutes. Short enough to reflect new listings within one cache cycle; long enough to absorb traffic spikes.

## Reasoning

The Philippines market is primarily urban — the top booking volume will come from Metro Manila, Cebu City, and Davao City, all of which are dense enough that a 4.9km cell will have many concurrent users hitting the same cache key. Precision-5 gives a good hit rate in these markets without making cells so large that a customer in Makati sees results from Pasig.

Precision-4 (~39km × 20km) would be too coarse — results across such a large area are not useful for a local booking. Precision-6 (~1.2km × 0.6km) would be too granular — cache hit rate drops significantly in lower-density areas.

## Consequences

- Use the `mmcloughlin/geohash` Go package for encoding: `geohash.Encode(lat, lng, 5)`
- The feed cache key must be invalidated actively — not just expired — when an owner publishes a new service, updates availability, or changes pricing. Invalidate all Geohash-5 cells whose center is within 10km of the updated branch:
  ```go
  // On owner service/availability update:
  cells := geohash.Neighbors(geohash.Encode(branch.Lat, branch.Lng, 5))
  cells = append(cells, geohash.Encode(branch.Lat, branch.Lng, 5))
  for _, cell := range cells {
      rdb.Del(ctx, fmt.Sprintf("feed:%s:*", cell))
  }
  ```
- The keyword search path (`/search?q=...`) bypasses the feed cache entirely and always hits PostgreSQL — the cache is only for the browse (no keyword) path
- In development, set the TTL to 10 seconds so changes are visible without waiting
- Do not cache results that include `deleted_at IS NOT NULL` rows — always filter before caching
