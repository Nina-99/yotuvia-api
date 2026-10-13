package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nina-99/yotuvia-api/controller"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// stubFavoriteService is an error-injecting FavoriteService used to prove the
// controller routes failures through apperrors.MapToHTTP instead of echoing
// err.Error() (which can carry DSNs, SQL and other internal text).
type stubFavoriteService struct {
	deleteErr error
	findErr   error
}

func (s *stubFavoriteService) Create(req request.CreateFavoriteRequest) error { return nil }

func (s *stubFavoriteService) Delete(id uint, actorClientID uint) error { return s.deleteErr }

func (s *stubFavoriteService) FindByClientID(clientID uint, actorClientID uint) ([]response.FavoriteResponse, error) {
	return nil, s.findErr
}

func (s *stubFavoriteService) IsFavorite(clientID, beautyCenterID uint) (bool, error) {
	return false, nil
}

// TestFavoriteControllerErrorMapping_NoInternalLeak — an unexpected service
// error must become a generic 500 {"error":"internal server error"}; the raw
// message (a fake DSN failure) must never reach the client.
func TestFavoriteControllerErrorMapping_NoInternalLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	utils.Log = zap.NewNop().Sugar()

	rawErr := errors.New(`pq: password authentication failed for user "yotuvia" (dsn leak)`)

	t.Run("delete maps an internal error to a generic 500", func(t *testing.T) {
		ctrl := controller.NewFavoriteController(&stubFavoriteService{deleteErr: rawErr})
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{{Key: "id", Value: "1"}}

		ctrl.Delete(ctx)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "internal server error", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), "password authentication")
	})

	t.Run("find maps an internal error to a generic 500", func(t *testing.T) {
		ctrl := controller.NewFavoriteController(&stubFavoriteService{findErr: rawErr})
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{{Key: "client_id", Value: "1"}}

		ctrl.FindByClientID(ctx)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "internal server error", errBody(t, w.Body.String()))
		assert.NotContains(t, w.Body.String(), "password authentication")
	})
}
