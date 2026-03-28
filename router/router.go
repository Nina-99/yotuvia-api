package router

import (
	"log"

	"github.com/Nina-99/yottu-api/configs"
	"github.com/Nina-99/yottu-api/controller"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	validate := validator.New()
	clientRepository := repository.NewClientRepositoryImpl(configs.DB)
	clientService := service.NewClientServiceImpl(clientRepository, validate)
	clientController := controller.NewClientController(clientService)
	categoryRepository := repository.NewCategoryRepositoryImpl(configs.DB)
	categoryService := service.NewCategoryServiceImpl(categoryRepository, validate)
	categoryController := controller.NewCategoryController(categoryService)
	reservationRepository := repository.NewReservationRepositoryImpl(configs.DB)
	reservationService := service.NewReservationServiceImpl(reservationRepository, validate)
	reservationController := controller.NewReservationController(reservationService)
	serviceRepository := repository.NewServiceRepositoryImpl(configs.DB)
	serviceService := service.NewServiceServiceImpl(serviceRepository, categoryRepository, validate)
	serviceController := controller.NewServiceController(serviceService)
	beautyCenterRepository := repository.NewBeautyCenterRepositoryImpl(configs.DB)
	beautyCenterService := service.NewBeautyCenterServiceImpl(beautyCenterRepository)
	beautyCenterController := controller.NewBeautyCenterController(beautyCenterService)
	personnelRepository := repository.NewPersonnelRepositoryImpl(configs.DB)
	personnelService := service.NewPersonnelServiceImpl(personnelRepository)
	personnelController := controller.NewPersonnelController(personnelService)

	personnelImageRepository := repository.NewPersonnelImageRepositoryImpl(configs.DB)
	personnelImageService := service.NewPersonnelImageServiceImpl(personnelImageRepository, personnelRepository, validate)
	personnelImageController := controller.NewPersonnelImageController(personnelImageService)

	membershipRepository := repository.NewMembershipRepositoryImpl(configs.DB)
	membershipService := service.NewMembershipService(membershipRepository)
	membershipController := controller.NewMembershipController(membershipService)

	authAccountRepository := repository.NewAuthAccountRepositoryImpl(configs.DB)
	authAccountService := service.NewAuthAccountServiceImpl(authAccountRepository, validate)
	authAccountController := controller.NewAuthAccountController(authAccountService)

	favoriteRepository := repository.NewFavoriteRepositoryImpl(configs.DB)
	favoriteService := service.NewFavoriteServiceImpl(favoriteRepository, validate)
	favoriteController := controller.NewFavoriteController(favoriteService)

	reviewRepository := repository.NewReviewRepositoryImpl(configs.DB)
	reviewService := service.NewReviewServiceImpl(reviewRepository)
	reviewController := controller.NewReviewController(reviewService)

	feedPostRepository := repository.NewFeedPostRepositoryImpl(configs.DB)
	feedPostService := service.NewFeedPostServiceImpl(feedPostRepository, beautyCenterRepository, validate)
	feedPostController := controller.NewFeedPostController(feedPostService)

	postLikeRepository := repository.NewPostLikeRepositoryImpl(configs.DB)
	postLikeService := service.NewPostLikeServiceImpl(postLikeRepository)
	postLikeController := controller.NewPostLikeController(postLikeService)

	socialMediaRepository := repository.NewSocialMediaRepositoryImpl(configs.DB)
	socialMediaService := service.NewSocialMediaServiceImpl(socialMediaRepository, validate)
	socialMediaController := controller.NewSocialMediaController(socialMediaService)

	api := r.Group("/api")
	{
		CategoryRoutes(api, categoryController)
		ClientRoutes(api, clientController)
		ReservationRoutes(api, reservationController)
		ServiceRoutes(api, serviceController)
		BeautyCenterRoutes(api, beautyCenterController)
		PersonnelRoutes(api, personnelController)
		PersonnelImageRoutes(api, personnelImageController)
		AuthAccountRoutes(api, authAccountController)
		MembershipRoutes(api, membershipController)
		FavoriteRouter(api, favoriteController)
		ReviewRouter(api, reviewController)
		FeedPostRoutes(api, feedPostController)
		PostLikeRoutes(api, postLikeController)
		SocialMediaRoutes(api, socialMediaController)
	}
	return r
}

func Run(addr string) {
	r := SetupRouter()
	log.Println("🚀 Server running on", addr)

	r.Run(addr)
}
