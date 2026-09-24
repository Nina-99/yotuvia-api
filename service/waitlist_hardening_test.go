package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/apperrors"
	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedWaitlistEntry inserts a queue row directly with an explicit
// created_at so FIFO ordering is deterministic (Join uses time.Now and two
// fast inserts can share a timestamp on SQLite).
func seedWaitlistEntry(t *testing.T, db *gorm.DB, centerID, clientID, personnelID uint, date time.Time, startTime, status string, createdAt time.Time) *models.WaitlistEntry {
	t.Helper()
	e := &models.WaitlistEntry{
		BeautyCenterID: centerID,
		ClientID:       clientID,
		Date:           date,
		StartTime:      startTime,
		EndTime:        "11:00",
		PersonnelID:    personnelID,
		ServiceIDs:     "1",
		Status:         status,
		CreatedAt:      createdAt,
	}
	require.NoError(t, db.Create(e).Error)
	return e
}

// TestWaitlistPositionsAreFIFORanks proves the live listing carries a 1-based
// FIFO rank per slot computed in one batched query: the earliest WAITING row
// ranks 1, the next 2, and an OFFERED row always holds rank 1.
func TestWaitlistPositionsAreFIFORanks(t *testing.T) {
	db, _, waitSvc, centerID, alice, bob, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)

	slot := time.Now().AddDate(0, 0, 8)
	t0 := time.Now().Add(-2 * time.Hour)
	aliceEntry := seedWaitlistEntry(t, db, centerID, alice, staff.ID, slot, "10:00", models.WaitlistWaiting, t0)
	bobEntry := seedWaitlistEntry(t, db, centerID, bob, staff.ID, slot, "10:00", models.WaitlistWaiting, t0.Add(time.Minute))

	aliceList, err := waitSvc.ListMine(alice, false)
	require.NoError(t, err)
	require.Len(t, aliceList, 1)
	assert.Equal(t, 1, aliceList[0].Position, "earliest waiter holds rank 1")

	bobList, err := waitSvc.ListMine(bob, false)
	require.NoError(t, err)
	require.Len(t, bobList, 1)
	assert.Equal(t, 2, bobList[0].Position, "second waiter holds rank 2")

	// Pushing alice's row to OFFERED keeps her at the front, bob stays 2.
	require.NoError(t, db.Model(&models.WaitlistEntry{}).Where("id = ?", aliceEntry.ID).
		Updates(map[string]any{"status": models.WaitlistOffered, "offer_expires_at": time.Now().Add(time.Hour)}).Error)

	aliceList, err = waitSvc.ListMine(alice, false)
	require.NoError(t, err)
	assert.Equal(t, 1, aliceList[0].Position, "OFFERED holds rank 1")

	bobList, err = waitSvc.ListMine(bob, false)
	require.NoError(t, err)
	assert.Equal(t, 2, bobList[0].Position, "bob stays behind the offer holder")

	// Sanity: the seeded IDs are the ones we ranked.
	assert.Equal(t, aliceEntry.ID, aliceList[0].ID)
	assert.Equal(t, bobEntry.ID, bobList[0].ID)
}

// TestWaitlistPositionsAreScopedPerSlot proves the batched rank query keeps
// slots independent and the live-status filter inside the OR group (a closed
// row from another slot must not influence a rank).
func TestWaitlistPositionsAreScopedPerSlot(t *testing.T) {
	db, _, waitSvc, centerID, alice, bob, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)

	slotA := time.Now().AddDate(0, 0, 13)
	slotB := time.Now().AddDate(0, 0, 14)
	base := time.Now().Add(-4 * time.Hour)

	// Slot A queue: alice first, bob second.
	seedWaitlistEntry(t, db, centerID, alice, staff.ID, slotA, "09:00", models.WaitlistWaiting, base)
	seedWaitlistEntry(t, db, centerID, bob, staff.ID, slotA, "09:00", models.WaitlistWaiting, base.Add(time.Minute))
	// Slot B: bob's closed row predates alice's live row. If the live-status
	// filter leaked out of the OR group it would count bob's BOOKED row and
	// push alice to rank 2.
	seedWaitlistEntry(t, db, centerID, bob, staff.ID, slotB, "09:00", models.WaitlistBooked, base)
	aliceB := seedWaitlistEntry(t, db, centerID, alice, staff.ID, slotB, "09:00", models.WaitlistWaiting, base.Add(time.Minute))

	aliceList, err := waitSvc.ListMine(alice, false)
	require.NoError(t, err)
	require.Len(t, aliceList, 2)
	for i := range aliceList {
		if aliceList[i].ID == aliceB.ID {
			assert.Equal(t, 1, aliceList[i].Position, "slot B rank ignores the closed row")
		} else {
			assert.Equal(t, 1, aliceList[i].Position, "slot A rank is independent")
		}
	}
}

// TestWaitlistHistoryReturnsClosedEntries proves history=true returns every
// status newest-first with Position 0 for closed rows and a real rank for the
// live one, while the default live listing still hides closed rows.
func TestWaitlistHistoryReturnsClosedEntries(t *testing.T) {
	db, _, waitSvc, centerID, alice, _, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)

	slot := time.Now().AddDate(0, 0, 9)
	base := time.Now().Add(-3 * time.Hour)
	live := seedWaitlistEntry(t, db, centerID, alice, staff.ID, slot, "10:00", models.WaitlistWaiting, base)
	closed := seedWaitlistEntry(t, db, centerID, alice, staff.ID, slot, "11:00", models.WaitlistCancelled, base.Add(time.Minute))
	booked := seedWaitlistEntry(t, db, centerID, alice, staff.ID, slot, "12:00", models.WaitlistBooked, base.Add(2*time.Minute))

	// Backward-compatible default: live only.
	liveList, err := waitSvc.ListMine(alice, false)
	require.NoError(t, err)
	require.Len(t, liveList, 1)
	assert.Equal(t, live.ID, liveList[0].ID)
	assert.Equal(t, 1, liveList[0].Position)

	history, err := waitSvc.ListMine(alice, true)
	require.NoError(t, err)
	require.Len(t, history, 3)
	// Newest first.
	assert.Equal(t, booked.ID, history[0].ID)
	assert.Equal(t, closed.ID, history[1].ID)
	assert.Equal(t, live.ID, history[2].ID)
	// Closed rows report 0; the live one reports its rank.
	assert.Equal(t, 0, history[0].Position, "BOOKED is closed")
	assert.Equal(t, 0, history[1].Position, "CANCELLED is closed")
	assert.Equal(t, 1, history[2].Position, "WAITING keeps its rank")
}

// TestWaitlistReadTimeExpiryCascadesToNextWaiter proves a lapsed OFFERED row
// is expired during the owner's live listing and the slot is passed to the
// next waiter before the response is built.
func TestWaitlistReadTimeExpiryCascadesToNextWaiter(t *testing.T) {
	db, _, waitSvc, centerID, alice, bob, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)

	slot := time.Now().AddDate(0, 0, 10)
	parsed, _ := time.Parse("2006-01-02", slot.Format("2006-01-02"))
	t0 := time.Now().Add(-2 * time.Hour)
	require.NoError(t, db.Create(&models.WaitlistEntry{
		BeautyCenterID: centerID, ClientID: alice, Date: slot, StartTime: "10:00",
		EndTime: "11:00", PersonnelID: staff.ID, ServiceIDs: "1",
		Status: models.WaitlistWaiting, CreatedAt: t0,
	}).Error)
	require.NoError(t, db.Create(&models.WaitlistEntry{
		BeautyCenterID: centerID, ClientID: bob, Date: slot, StartTime: "10:00",
		EndTime: "11:00", PersonnelID: staff.ID, ServiceIDs: "1",
		Status: models.WaitlistWaiting, CreatedAt: t0.Add(time.Minute),
	}).Error)

	first, err := waitSvc.OfferForFreedSlot(centerID, parsed, "10:00")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, alice, first.ClientID)

	// Lapse the window.
	past := time.Now().Add(-time.Minute)
	require.NoError(t, db.Model(&models.WaitlistEntry{}).Where("id = ?", first.ID).
		Update("offer_expires_at", past).Error)

	// Alice's own live read expires her offer and cascades.
	aliceList, err := waitSvc.ListMine(alice, false)
	require.NoError(t, err)
	assert.Empty(t, aliceList, "the lapsed offer must not ship as live/claimable")

	var aliceRow models.WaitlistEntry
	require.NoError(t, db.First(&aliceRow, first.ID).Error)
	assert.Equal(t, models.WaitlistExpired, aliceRow.Status, "row transitions to EXPIRED at read time")

	// Bob is now holding the offer.
	bobList, err := waitSvc.ListMine(bob, false)
	require.NoError(t, err)
	require.Len(t, bobList, 1)
	assert.Equal(t, models.WaitlistOffered, bobList[0].Status)
	assert.Equal(t, 1, bobList[0].Position)
}

// TestWaitlistLapsedOfferIsNotClaimable proves that once read-time expiry has
// run, the stale offer cannot be claimed as if it were still OFFERED.
func TestWaitlistLapsedOfferIsNotClaimable(t *testing.T) {
	db, _, waitSvc, centerID, alice, _, _ := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)

	slot := time.Now().AddDate(0, 0, 11)
	parsed, _ := time.Parse("2006-01-02", slot.Format("2006-01-02"))
	require.NoError(t, db.Create(&models.WaitlistEntry{
		BeautyCenterID: centerID, ClientID: alice, Date: slot, StartTime: "10:00",
		EndTime: "11:00", PersonnelID: staff.ID, ServiceIDs: "1",
		Status: models.WaitlistWaiting, CreatedAt: time.Now().Add(-time.Hour),
	}).Error)

	first, err := waitSvc.OfferForFreedSlot(centerID, parsed, "10:00")
	require.NoError(t, err)
	require.NotNil(t, first)

	past := time.Now().Add(-time.Minute)
	require.NoError(t, db.Model(&models.WaitlistEntry{}).Where("id = ?", first.ID).
		Update("offer_expires_at", past).Error)

	// Trigger read-time expiry through the listing, then attempt to claim the
	// stale offer: it must be rejected, never treated as a live OFFERED row.
	_, err = waitSvc.ListMine(alice, false)
	require.NoError(t, err)

	_, claimErr := waitSvc.Claim(first.ID, alice)
	assert.ErrorIs(t, claimErr, apperrors.ErrConflict, "a lapsed offer is not claimable")

	var row models.WaitlistEntry
	require.NoError(t, db.First(&row, first.ID).Error)
	assert.NotEqual(t, models.WaitlistOffered, row.Status)
}

// TestWaitlistOfferPayloadIsFlat pins contracts #4/#5: the waitlist_offer push
// and the in-app notification data are the SAME flat JSON object (no
// double-encoding, no nested data string, and type present on both).
func TestWaitlistOfferPayloadIsFlat(t *testing.T) {
	db, resSvc, _, centerID, alice, _, serviceID := setupPhase3TestDB(t)
	var staff models.Personnel
	require.NoError(t, db.First(&staff).Error)

	notif := &fakeNotificationService{}
	waitSvc := NewWaitlistServiceImpl(
		repository.NewWaitlistRepositoryImpl(db),
		repository.NewReservationRepositoryImpl(db),
		resSvc,
		repository.NewServiceRepositoryImpl(db),
		repository.NewClientRepositoryImpl(db),
		notif,
		validator.New(),
	)

	slotDate := time.Now().AddDate(0, 0, 12).Format("2006-01-02")
	_, err := waitSvc.Join(request.JoinWaitlistRequest{
		BeautyCenterID: centerID,
		Date:           slotDate,
		StartTime:      "10:00",
		PersonnelID:    staff.ID,
		ServiceIDs:     []uint{serviceID},
	}, alice)
	require.NoError(t, err)

	parsed, _ := time.Parse("2006-01-02", slotDate)
	entry, err := waitSvc.OfferForFreedSlot(centerID, parsed, "10:00")
	require.NoError(t, err)
	require.NotNil(t, entry)

	require.Len(t, notif.pushedData, 1, "one push must be sent")
	var push map[string]any
	require.NoError(t, json.Unmarshal([]byte(notif.pushedData[0]), &push), "push payload must be valid flat JSON")
	assert.Equal(t, "waitlist_offer", push["type"])
	assert.Equal(t, float64(entry.ID), push["waitlist_id"])
	assert.Equal(t, slotDate, push["reservation_date"])
	assert.Equal(t, "10:00", push["start_time"])
	_, nested := push["data"]
	assert.False(t, nested, "push payload must not be double-encoded under data")

	// In-app data is the same flat object and also carries the type.
	require.Len(t, notif.created, 1, "one in-app notification must be written")
	assert.JSONEq(t, notif.pushedData[0], notif.created[0])
	var inApp map[string]any
	require.NoError(t, json.Unmarshal([]byte(notif.created[0]), &inApp))
	assert.Equal(t, "waitlist_offer", inApp["type"], "in-app data must include type for deep-linking")
}
