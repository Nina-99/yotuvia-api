package controller

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
)

// Ad density: how many organic items sit between two promoted slots. It is
// configurable because it is a product decision (a richer feed tolerates more
// ads than a thin one) but BOUNDED, so a typo can neither turn the feed into a
// billboard (1) nor silently disable the product (a huge number).
const (
	defaultBoostSlotDensity = 5
	minBoostSlotDensity     = 3
	maxBoostSlotDensity     = 20
)

// boostSlotDensity resolves BOOST_SLOT_DENSITY, clamped to the safe range.
func boostSlotDensity() int {
	raw := os.Getenv("BOOST_SLOT_DENSITY")
	if raw == "" {
		return defaultBoostSlotDensity
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultBoostSlotDensity
	}
	if n < minBoostSlotDensity {
		return minBoostSlotDensity
	}
	if n > maxBoostSlotDensity {
		return maxBoostSlotDensity
	}
	return n
}

type VideoWatchEventController struct {
	watchEventService service.VideoWatchEventService
	feedPostService   service.FeedPostService
	boostService      service.BoostService
}

func NewVideoWatchEventController(watchSvc service.VideoWatchEventService, feedSvc service.FeedPostService, boostSvc service.BoostService) *VideoWatchEventController {
	return &VideoWatchEventController{
		watchEventService: watchSvc,
		feedPostService:   feedSvc,
		boostService:      boostSvc,
	}
}

// RecordWatch handles POST /video-watch-events
func (c *VideoWatchEventController) RecordWatch(ctx *gin.Context) {
	var req request.RecordWatchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeBadRequest(ctx, err)
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := c.watchEventService.RecordWatch(clientID.(uint), req); err != nil {
		writeInternalError(ctx, err)
		return
	}

	// Billing: a boosted post reports its signed impression token here and the
	// charge happens only if the view qualifies. Best-effort — a billing failure
	// must never fail the watch record.
	if req.ImpressionToken != "" && c.boostService != nil {
		_ = c.boostService.SettleImpression(clientID.(uint), req.ImpressionToken, req.WatchPercentage, req.Completed)
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "watch recorded"})
}

// GetPersonalizedFeed handles GET /feed-posts/personalized
func (c *VideoWatchEventController) GetPersonalizedFeed(ctx *gin.Context) {
	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get all posts to match against personalized IDs (with clientID for is_liked/is_saved)
	clientIDUint := clientID.(uint)
	allPosts, _, err := c.feedPostService.FindAll(1, 1000, &clientIDUint)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	// Build lookup map
	postMap := make(map[uint]response.FeedPostResponse, len(allPosts))
	for _, p := range allPosts {
		postMap[p.ID] = p
	}

	// Viewer position: optional. With it, nearby content and nearby ads win;
	// without it, everything behaves exactly as before.
	viewer := viewerFromQuery(ctx)

	// Pagination over the SAME ranking: page 2 is not a different feed.
	limit, offset := personalizedWindow(ctx)
	session := ctx.Query("session")

	ids, rankedTotal, err := c.watchEventService.GetPersonalizedFeed(clientIDUint, limit, offset, viewer, session)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}

	// Promoted slots: auction over eligible campaigns, capped at one per
	// boostSlotDensity positions. Serving a slot NO LONGER bills it: the winning
	// post carries a signed impression token and the charge happens only when the
	// client reports a qualified view.
	promotedIDs, impressionTokens := c.servePromotedPosts(ids, postMap, viewer, clientIDUint)

	ordered, promotedSet := service.MergePromotedIDs(ids, promotedIDs, boostSlotDensity())

	// Collect posts in ranked order, labelling the promoted ones.
	posts := make([]response.FeedPostResponse, 0, len(ordered))
	for _, id := range ordered {
		if p, ok := postMap[id]; ok {
			p.IsBoosted = promotedSet[id]
			if token, ok := impressionTokens[id]; ok {
				p.ImpressionToken = token
			}
			posts = append(posts, p)
		}
	}

	// Serve-level instrumentation: only the first page, only a bounded number of
	// rows. This is the baseline that any future ranking change is measured
	// against — without it, "the feed feels better" is an opinion.
	if offset == 0 {
		c.recordImpressions(clientIDUint, session, posts)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"posts":  posts,
		"total":  rankedTotal,
		"offset": offset,
		"limit":  limit,
	})
}

// servePromotedPosts runs the boost auction for one page and returns the
// promoted post ids that are renderable here plus a signed impression token per
// slot. It intentionally does NOT settle: billing is deferred to the qualified
// view report. A nil boost service (feature off) yields no promoted slots.
func (c *VideoWatchEventController) servePromotedPosts(ids []uint, postMap map[uint]response.FeedPostResponse, viewer *service.GeoPoint, viewerID uint) ([]uint, map[uint]string) {
	if c.boostService == nil {
		return nil, nil
	}
	density := boostSlotDensity()
	slots := len(ids) / density
	if slots <= 0 {
		return nil, nil
	}
	winners, err := c.boostService.SelectWinners(time.Now(), slots, 0, viewer, viewerID)
	if err != nil {
		return nil, nil
	}
	// Serve-level frequency control: never promote a video this viewer already
	// watched. It is the one signal we have without writing a row per scroll,
	// and it is honest — re-serving a consumed ad annoys the viewer and adds no
	// reach for the advertiser.
	winnerPosts := make([]uint, 0, len(winners))
	for _, w := range winners {
		winnerPosts = append(winnerPosts, w.PostID)
	}
	watched, watchErr := c.watchEventService.WatchedPostIDs(viewerID, winnerPosts)
	if watchErr != nil {
		watched = nil // measurement failure must never suppress every ad
	}

	promoted := make([]uint, 0, len(winners))
	tokens := make(map[uint]string, len(winners))
	unserved := make([]uint, 0, 1)
	for _, winner := range winners {
		if watched[winner.PostID] {
			// Deliberate skip, not a delivery failure: the viewer consumed it.
			continue
		}
		if _, ok := postMap[winner.PostID]; !ok {
			// Won the auction, nothing to render: book the delivery miss so the
			// advertiser can see a campaign that never reaches anyone.
			unserved = append(unserved, winner.CampaignID)
			continue
		}
		promoted = append(promoted, winner.PostID)
		if token, err := c.boostService.ImpressionToken(winner.CampaignID, winner.PriceCPMMilli); err == nil {
			tokens[winner.PostID] = token
		}
	}
	// Best-effort: a counter write must never fail the feed.
	_ = c.boostService.RecordUnservedWins(unserved)

	return promoted, tokens
}

// viewerFromQuery reads optional `lat`/`lng` query parameters.
//
// Both must be present and plausible; a half-specified or out-of-range pair is
// treated as "no position" rather than as a real point, so a malformed client
// degrades to the pre-geo ranking instead of ranking everyone as very far.
func viewerFromQuery(ctx *gin.Context) *service.GeoPoint {
	rawLat, rawLng := ctx.Query("lat"), ctx.Query("lng")
	if rawLat == "" || rawLng == "" {
		return nil
	}
	lat, err := strconv.ParseFloat(rawLat, 64)
	if err != nil {
		return nil
	}
	lng, err := strconv.ParseFloat(rawLng, 64)
	if err != nil {
		return nil
	}
	point := &service.GeoPoint{Latitude: lat, Longitude: lng}
	if !point.Valid() {
		return nil
	}
	return point
}

// personalizedWindow parses `limit`/`offset` for the ranked feed.
//
// `limit` is clamped (1..50): the ranking is computed over the whole candidate
// set anyway, so a huge page only costs response size, but an unbounded one is
// still an easy way to make the server serialise the entire catalog.
func personalizedWindow(ctx *gin.Context) (limit, offset int) {
	limit, offset = 20, 0
	if raw := ctx.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 50 {
		limit = 50
	}
	if raw := ctx.Query("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			offset = n
		}
	}
	return limit, offset
}

// impressionLogLimit bounds how many served posts are logged per session. The
// first screen is where position bias actually shows; logging all 50 would pay
// for data nobody reads.
const impressionLogLimit = 20

// recordImpressions is best-effort: a logging failure must never fail a feed.
func (c *VideoWatchEventController) recordImpressions(clientID uint, session string, posts []response.FeedPostResponse) {
	if len(posts) == 0 {
		return
	}
	n := len(posts)
	if n > impressionLogLimit {
		n = impressionLogLimit
	}
	rows := make([]service.ImpressionRow, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, service.ImpressionRow{
			PostID: posts[i].ID, Position: i + 1, SessionID: session,
		})
	}
	if err := c.watchEventService.RecordImpressions(clientID, rows); err != nil {
		utils.Log.Warnf("[FEED] impression logging failed for client %d: %v", clientID, err)
	}
}

// FeedMetrics handles GET /admin/platform/feed/metrics (super_admin).
//
// Engagement per slot over the last `days` days: the CTR-like curve that says
// whether the top of the feed is actually better than the bottom.
func (c *VideoWatchEventController) FeedMetrics(ctx *gin.Context) {
	days := 7
	if raw := ctx.Query("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}

	metrics, err := c.watchEventService.PositionMetrics(days)
	if err != nil {
		writeInternalError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"days": days, "positions": metrics})
}
