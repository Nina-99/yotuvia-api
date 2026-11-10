package controller

import (
	"net/http"
	"time"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
)

// boostSlotDensity is how many organic items sit between two promoted slots:
// one promoted item per N positions (an ad-density cap so the feed never turns
// into a billboard).
const boostSlotDensity = 5

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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientID, exists := ctx.Get("clientID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if err := c.watchEventService.RecordWatch(clientID.(uint), req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build lookup map
	postMap := make(map[uint]response.FeedPostResponse, len(allPosts))
	for _, p := range allPosts {
		postMap[p.ID] = p
	}

	// Get personalized IDs
	ids, err := c.watchEventService.GetPersonalizedFeed(clientIDUint, 50)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Promoted slots: auction over eligible campaigns, capped at one per
	// boostSlotDensity positions. Serving a slot NO LONGER bills it: the winning
	// post carries a signed impression token and the charge happens only when the
	// client reports a qualified view.
	promotedIDs, impressionTokens := c.servePromotedPosts(ids, postMap)

	ordered, promotedSet := service.MergePromotedIDs(ids, promotedIDs, boostSlotDensity)

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

	ctx.JSON(http.StatusOK, gin.H{"posts": posts, "total": len(posts)})
}

// servePromotedPosts runs the boost auction for one page and returns the
// promoted post ids that are renderable here plus a signed impression token per
// slot. It intentionally does NOT settle: billing is deferred to the qualified
// view report. A nil boost service (feature off) yields no promoted slots.
func (c *VideoWatchEventController) servePromotedPosts(ids []uint, postMap map[uint]response.FeedPostResponse) ([]uint, map[uint]string) {
	if c.boostService == nil {
		return nil, nil
	}
	slots := len(ids) / boostSlotDensity
	if slots <= 0 {
		return nil, nil
	}
	winners, err := c.boostService.SelectWinners(time.Now(), slots, 0)
	if err != nil {
		return nil, nil
	}
	promoted := make([]uint, 0, len(winners))
	tokens := make(map[uint]string, len(winners))
	for _, winner := range winners {
		if _, ok := postMap[winner.PostID]; !ok {
			continue
		}
		promoted = append(promoted, winner.PostID)
		if token, err := c.boostService.ImpressionToken(winner.CampaignID, winner.PriceCPMMilli); err == nil {
			tokens[winner.PostID] = token
		}
	}
	return promoted, tokens
}
