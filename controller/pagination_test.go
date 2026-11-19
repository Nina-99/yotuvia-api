package controller

// Clamping `page`/`limit` is a security control: a negative limit reaches GORM
// as `Limit(-1)`, which is a no-op and drops the LIMIT clause entirely — a
// full-table dump in one request. These specs pin the contract so the next
// handler that forgets to use the helper fails here.

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func limitCtx(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x"+query, nil)
	return c
}

func TestPagination_ClampsBothDimensions(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantPage  int
		wantLimit int
	}{
		{"defaults", "", 1, 20},
		{"negative limit is a full dump if unclamped", "?limit=-1", 1, 20},
		{"zero limit", "?limit=0", 1, 20},
		{"huge limit", "?limit=1000000", 1, 100},
		{"negative page", "?page=-5", 1, 20},
		{"huge page becomes a bounded offset", "?page=999999999", maxPageIndex, 20},
		{"valid values pass through", "?page=3&limit=50", 3, 50},
		{"garbage falls back to defaults", "?page=abc&limit=xyz", 1, 20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, limit := pagination(limitCtx(tc.query))
			assert.Equal(t, tc.wantPage, page)
			assert.Equal(t, tc.wantLimit, limit)
			assert.Greater(t, limit, 0, "a non-positive limit must never reach the query builder")
		})
	}
}

func TestPaginationLimit_HonoursCustomBounds(t *testing.T) {
	page, limit := paginationLimit(limitCtx("?limit=999&page=2"), 10, 50)
	assert.Equal(t, 2, page)
	assert.Equal(t, 50, limit)

	// A ranking with no page parameter still gets page 1.
	page, limit = paginationLimit(limitCtx(""), 10, 50)
	assert.Equal(t, 1, page)
	assert.Equal(t, 10, limit, "the custom default must win over the shared 20")
}

func TestQueryLimit_Clamps(t *testing.T) {
	assert.Equal(t, 10, queryLimit(limitCtx("?limit=-1"), 10, 50))
	assert.Equal(t, 50, queryLimit(limitCtx("?limit=500"), 10, 50))
	assert.Equal(t, 7, queryLimit(limitCtx("?limit=7"), 10, 50))
}
