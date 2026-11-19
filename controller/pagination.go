package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	// maxPageIndex bounds OFFSET. Without it `?page=100000000` becomes
	// `OFFSET 2e9`, which makes Postgres read and discard millions of rows for
	// the price of one request — a 1:1000 DoS amplifier.
	maxPageIndex = 10_000
)

// pagination reads `page` and `limit` from the query and clamps BOTH.
//
// Clamping is a security control, not cosmetics:
//   - a negative or zero `limit` used to reach GORM, where `Limit(-1)` is a
//     no-op and the query runs WITHOUT a LIMIT (a full-table dump);
//   - an unbounded `limit` lets one request ask for the whole table;
//   - an unbounded `page` turns into a gigantic OFFSET.
//
// Every list endpoint must use this (or queryLimit/paginationLimit) instead of
// calling strconv.Atoi on the query directly, so the next handler cannot ship
// the bug the previous one had.
func pagination(ctx *gin.Context) (page, limit int) {
	return paginationLimit(ctx, defaultPageSize, maxPageSize)
}

// paginationLimit is pagination with an explicit default and cap, for endpoints
// that are not page-sized (a top-10 ranking, a 50-row preview).
func paginationLimit(ctx *gin.Context, defaultSize, maxLimit int) (page, limit int) {
	if defaultSize < 1 {
		defaultSize = defaultPageSize
	}
	if maxLimit < 1 {
		maxLimit = maxPageSize
	}

	page, _ = strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(ctx.DefaultQuery("limit", strconv.Itoa(defaultSize)))

	if page < 1 {
		page = 1
	}
	if page > maxPageIndex {
		page = maxPageIndex
	}
	if limit < 1 {
		limit = defaultSize
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit
}

// queryLimit clamps a `limit`-only query parameter (rankings and previews that
// have no page).
func queryLimit(ctx *gin.Context, defaultSize, maxLimit int) int {
	_, limit := paginationLimit(ctx, defaultSize, maxLimit)
	return limit
}
