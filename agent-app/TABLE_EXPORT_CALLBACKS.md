# Stable table exports

Large table exports can opt into `OnTableExportPlan` and `OnTableExportChunk` on `app.TableTemplate`.
The two callbacks must always be registered together.

`OnTableExportPlan` receives the current business filters and the requested chunk size. It should freeze membership (for example, capture the current maximum ID), count the matching rows, and return independently readable blocks. `Snapshot` and each block `Cursor` are opaque to kageos, so applications may encode signed IDs, time boundaries, or a temporary snapshot key.

`OnTableExportChunk` receives the unchanged snapshot, block cursor, limit, and filters. It returns the rows for exactly that block. Prefer keyset predicates such as:

```sql
WHERE id > :after_id AND id <= :snapshot_max_id
ORDER BY id ASC
LIMIT :limit
```

Do not use OFFSET inside this callback: concurrent inserts or deletes can move OFFSET pages. A plan callback should also validate or sign opaque snapshot/cursor values before using them in a query.

When both callbacks are present, the Table schema advertises `OnTableExportPlan` and `OnTableExportChunk`; the kageos export dialog then uses the stable callback path automatically. Tables without the callbacks continue to use the compatibility path with fixed ID ascending order.
