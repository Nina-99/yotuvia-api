package router

import (
	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

func LoyaltyRoutes(rg *gin.RouterGroup, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewLoyaltyRepositoryImpl(db)
	centerRepo := repository.NewBeautyCenterRepositoryImpl(db)
	svc := service.NewLoyaltyService(repo, centerRepo,
		service.NewTokenMissionServiceImpl(
			repository.NewTokenMissionRepositoryImpl(db), nil,
			repository.NewTokenAccountRepositoryImpl(db),
		), validate)
	ctrl := controller.NewLoyaltyController(svc)

	loyalty := rg.Group("/loyalty")

	// Every loyalty route is authenticated. Before this gate the whole group was
	// anonymous: anyone could create/update/delete center programs and read any
	// client's punch cards without a token.
	loyalty.Use(middleware.JWTAuthMiddleware())

	// Client-facing read, authenticated-only for EVERY role (no role gate):
	//   - the signed-in client fetches its own punch cards (mobile PerfilScreen);
	//   - the center admin inspects a client's cards (web LoyaltyPage via
	//     getClientLoyaltyCards in admin-api.ts).
	// Ownership is enforced in the service: a `client` token may only read the
	// clientID in its own claim (role + :clientId compare, 403 on mismatch).
	// beauty_center and personnel keep inspecting any client's cards.
	// Covered by tests/loyalty_authz_test.go TestLoyaltyClientCards_*.
	loyalty.GET("/client/:clientId", ctrl.GetCLientCards)
	loyalty.GET("/client/:clientId/history", ctrl.GetClientHistory)
	loyalty.GET("/client/:clientId/pending", ctrl.GetPendingRewards)

	// Anonymous public programs: a NEW route (never a widened admin route) so
	// guests see the earn badge before booking. Registered on rg (no JWT —
	// the loyalty group's gate does not apply to the parent).
	rg.GET("/loyalty/public/:centerId", ctrl.GetPublicPrograms)

	// Center program + settings management are beauty_center-only. The role gate
	// proves the ROLE; the centerID claim is verified against row ownership in
	// the service layer, so a center-A admin cannot touch center-B data.
	admin := loyalty.Group("")
	admin.Use(middleware.RequireRole("beauty_center"))
	{
		admin.POST("", ctrl.CreateProgram)
		admin.GET("/center/:centerId", ctrl.FindByCenter)
		admin.GET("/center/:centerId/settings", ctrl.GetCenterSettings)
		admin.PUT("/center/:centerId/settings", ctrl.UpdateCenterSettings)
		admin.GET("/center/:centerId/metrics", ctrl.GetMetrics)
		admin.PUT("/:id", ctrl.UpdateProgram)
		admin.DELETE("/:id", ctrl.DeleteProgram)
		admin.PATCH("/:id/toggle", ctrl.ToggleProgram)
	}
}
