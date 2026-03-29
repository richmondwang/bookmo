# Add a database migration

Create a new numbered migration pair for: $ARGUMENTS

## Steps

1. Find the highest existing migration number in `migrations/` by listing the directory
2. Create `migrations/NNN_<slug>.up.sql` and `migrations/NNN_<slug>.down.sql` where NNN is the next number (zero-padded to 3 digits)
3. Write the up migration using these conventions:
   - All PKs: `UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
   - All tables: include `created_at TIMESTAMP NOT NULL DEFAULT now()`
   - All mutable tables: include `deleted_at TIMESTAMP`
   - Monetary amounts: `INT` (centavos), never `NUMERIC` for money fields
   - Geography: `GEOGRAPHY(POINT, 4326)` not `GEOMETRY`
   - Indexes: always add indexes for FK columns and frequently filtered columns
   - Partial indexes for soft-deleted tables: `WHERE deleted_at IS NULL`
4. Write the down migration that cleanly reverses every change (DROP in reverse order)
5. Confirm both files look correct and print their contents

## Do not

- Use SERIAL or integer PKs
- Use hard DELETE patterns
- Store amounts as NUMERIC/DECIMAL (use INT centavos)
- Forget to add `WHERE deleted_at IS NULL` on indexes for soft-deleted tables