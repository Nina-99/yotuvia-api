package tests

import (
	"errors"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingMission always errors, proving the publish path stays independent.
type failingMission struct{}

func (failingMission) TriggerCheck(uint, string) error { return errors.New("boom") }
func (failingMission) ClaimReward(uint, uint) error    { return nil }
func (failingMission) GetProgress(uint) ([]models.TokenMissionProgress, error) {
	return nil, nil
}
func (failingMission) GetAvailableMissions() ([]models.TokenMission, error) { return nil, nil }

// TestFeed_CreateExternal_TriggersVideoPublished — publishing a video advances
// the weekly video mission once.
func TestFeed_CreateExternal_TriggersVideoPublished(t *testing.T) {
	mission := &stubMission{}
	svc := newExternalFeedSUT(t, &stubResolver{}, mission)

	_, err := svc.CreateFromExternal(staffActor, request.CreateExternalFeedPostRequest{
		ExternalURL: "https://www.tiktok.com/@studio/video/7312345678",
		Caption:     "Look",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, mission.triggerCalls)
}

// TestFeed_CreateExternal_MissionFailureDoesNotBlock — a mission engine error is
// best-effort and never fails the post.
func TestFeed_CreateExternal_MissionFailureDoesNotBlock(t *testing.T) {
	svc := newExternalFeedSUT(t, &stubResolver{}, failingMission{})

	res, err := svc.CreateFromExternal(staffActor, request.CreateExternalFeedPostRequest{
		ExternalURL: "https://www.tiktok.com/@studio/video/7312345678",
		Caption:     "Look",
	})
	require.NoError(t, err)
	assert.NotZero(t, res.ID)
}
