# Video → Booking Funnel Metrics

Answer "where is the leak?" from the database today — no dashboard required.
Every query below was executed against the local Postgres (`docker exec -it yotuvia_db psql -U yotuvia -d yotuvia`)
with the schema described here.

> **Snapshot caveat.** The attribution columns (`reservations.source_post_id`,
> `feed_posts.book_taps`) are new. Since the feature just shipped, the funnel
> query currently returns zero taps and zero attributed bookings. The waitlist
> table is empty in this dataset. Both were proven correct with a **rolled-back
> sample** (results included below), so the SQL is verified, not aspirational.

## Tables and columns relied on

| Table | Columns used | Why |
|-------|--------------|-----|
| `feed_posts` | `id`, `beauty_center_id`, `views_count`, `book_taps`, `deleted_at` | the video unit and its top-of-funnel counters |
| `reservations` | `id`, `client_id`, `source_post_id`, `status`, `created_at`, `deleted_at` | attributed bookings / rebook signal |
| `waitlist_entries` | `status`, `offered_at`, `created_at`, `deleted_at` | middle funnel: interest before commitment |

`source_post_id` and `book_taps` are nullable/default-0 and **additive**: every
existing row keeps working. A `source_post_id` that does not resolve is dropped
at write time, so the column is already clean — no extra filtering needed.
Soft-deleted rows must be excluded with `deleted_at IS NULL` (GORM soft delete).

---

## 1. Video funnel — per post

**What it means:** for each video, how many people watched, how many tapped the
book CTA, and how many actually booked. `bookings_per_view` is the end-to-end
conversion; `bookings_per_tap` is the CTA's own conversion (how good the offer
is once interest exists). Compare `views → taps` (is the CTA compelling?) with
`taps → bookings` (does the booking flow deliver?).

```sql
SELECT
    fp.id                                                      AS post_id,
    fp.beauty_center_id                                        AS center_id,
    fp.views_count,
    fp.book_taps,
    COUNT(r.id)                                                AS attributed_bookings,
    ROUND(COUNT(r.id)::numeric / NULLIF(fp.views_count, 0), 4)  AS bookings_per_view,
    ROUND(COUNT(r.id)::numeric / NULLIF(fp.book_taps, 0), 4)    AS bookings_per_tap
FROM feed_posts fp
LEFT JOIN reservations r
       ON r.source_post_id = fp.id
      AND r.deleted_at IS NULL
WHERE fp.deleted_at IS NULL
GROUP BY fp.id, fp.beauty_center_id, fp.views_count, fp.book_taps
ORDER BY fp.views_count DESC
LIMIT 20;
```

**How to read it:** `bookings_per_tap` is NULL when nobody tapped (guard, not a
bug). A high view count with zero taps means the video entertains but the CTA
does not sell; taps with no bookings means the offer/booking path leaks.

**Result obtained (real data, top rows):** every post returned
`book_taps = 0`, `attributed_bookings = 0`, `bookings_per_view = 0.0000`,
`bookings_per_tap = NULL` — expected, the feature is brand new. E.g.
`post 43 · center 17 · views 2053 · taps 0 · bookings 0`.

**Proven with a rolled-back sample** (post 9, 707 views, 120 taps, 12 attributed
bookings):

| post_id | views_count | book_taps | attributed_bookings | bookings_per_view | bookings_per_tap |
|---------|-------------|-----------|---------------------|-------------------|------------------|
| 9 | 707 | 120 | 12 | 0.0170 | 0.1000 |

---

## 2. Waitlist claim rate — interest before commitment

**What it means:** how many clients joined a full slot's queue, how many were
actually offered the freed slot, and how many claimed it. The claim rate is the
share of offers that converted. `joined` that never becomes `offered` is a
supply problem (slots never free up); `offered` that never becomes `booked` is a
demand/notification problem (people ignore the offer or the window is too short).

```sql
SELECT
    COUNT(*)                                                     AS joined,
    COUNT(*) FILTER (WHERE offered_at IS NOT NULL)               AS offered,
    COUNT(*) FILTER (WHERE status = 'BOOKED')                    AS booked,
    COUNT(*) FILTER (WHERE status = 'EXPIRED')                   AS expired,
    ROUND(
      COUNT(*) FILTER (WHERE status = 'BOOKED')::numeric
      / NULLIF(COUNT(*) FILTER (WHERE offered_at IS NOT NULL), 0), 4
    )                                                            AS claim_rate
FROM waitlist_entries
WHERE deleted_at IS NULL;
```

**How to read it:** `offered` = entries that ever received an offer
(`offered_at IS NOT NULL`), so it rolls up `BOOKED` + currently-`OFFERED` +
`EXPIRED`. `claim_rate = booked / offered`. Add
`AND created_at >= NOW() - INTERVAL '30 days'` for a period, or
`GROUP BY beauty_center_id` to rank centers.

**Result obtained (real data):** table empty → `joined 0 · offered 0 · booked 0
· expired 0 · claim_rate NULL`. The query executes cleanly.

**Proven with a rolled-back sample** (6 booked, 3 expired, 1 waiting):

| joined | offered | booked | expired | claim_rate |
|--------|---------|--------|---------|------------|
| 10 | 9 | 6 | 3 | 0.6667 |

---

## 3. Rebook rate — does a client come back?

**What it means:** among clients with at least one completed reservation, what
share has two or more. This is the retention/rebook signal and the healthiest
long-term metric — a one-time booking is a transaction, a rebooking is a
relationship.

```sql
WITH per_client AS (
    SELECT client_id, COUNT(*) AS completed_count
    FROM reservations
    WHERE deleted_at IS NULL
      AND UPPER(status) = 'COMPLETED'
    GROUP BY client_id
)
SELECT
    COUNT(*) FILTER (WHERE completed_count >= 1) AS clients_with_1plus,
    COUNT(*) FILTER (WHERE completed_count >= 2) AS clients_with_2plus,
    ROUND(
      COUNT(*) FILTER (WHERE completed_count >= 2)::numeric
      / NULLIF(COUNT(*) FILTER (WHERE completed_count >= 1), 0), 4
    ) AS rebook_rate
FROM per_client;
```

**How to read it:** `UPPER(status)` covers legacy lowercase rows. A high
`rebook_rate` means the base is loyal; watch it per center by adding
`beauty_center_id` to the CTE and grouping the outer query. For a stricter
definition ("came back after the first visit"), require the second completed
reservation's `created_at` to be later than the first.

**Result obtained (real data):**

| clients_with_1plus | clients_with_2plus | rebook_rate |
|--------------------|--------------------|-------------|
| 16 | 16 | 1.0000 |

(All 16 clients with a completed reservation have 2+ — expected for seeded demo
data.)

---

## Already measurable (no new work needed)

These need no new columns or endpoints; they are listed so nobody builds
infrastructure for them:

- **Engagement** — `feed_posts.views_count`, `post_likes`, `saved_videos`,
  `video_watch_events` (`watch_duration`, `watch_percentage`, `completed`).
- **Waitlist status mix** — `waitlist_entries.status` + `offered_at`.
- **Rebook** — `reservations` per `client_id` over time.

The only previously missing links were **attribution**
(`reservations.source_post_id`) and the **middle funnel step**
(`feed_posts.book_taps`), which is exactly what these queries consume.
