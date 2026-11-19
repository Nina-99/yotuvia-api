package repository

// ListQueryLimit is the server-side ceiling for user-facing LIST queries that
// do not expose pagination. Without it a long-lived account (likes, saves,
// watch history) or a viral post (public likes list) could return an unbounded
// result set to the mobile client.
//
// Every capped finder keeps its existing ordering (created_at DESC, or
// created_at DESC), so the NEWEST rows survive the cut.
//
// It deliberately does NOT apply to:
//   - COUNT(*) methods (they return a scalar, not a list);
//   - ID-scoped batch helpers (CountByFeedPostIDs, FindByIDs) that only ever
//     look at an explicit, caller-bounded id list;
//   - the parameterised FindAllActive, whose internal personalized-feed scan
//     legitimately needs up to 1000 rows — that is an internal scan, not a
//     user-facing list.
const ListQueryLimit = 200
