package configs

import (
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
)

// ReservationPersonnelBackfillStats reports the outcome of the best-effort,
// idempotent line-level professional backfill.
type ReservationPersonnelBackfillStats struct {
	// Eligible is the number of reservation service rows that still had a NULL
	// personnel_id when the backfill started. Zero on a second run (no-op).
	Eligible int
	// Filled is the number of rows actually updated to the reservation's lead.
	Filled int
	// Skipped counts rows left untouched because the reservation row was not
	// found (orphan) or the update failed; they are logged, never fatal.
	Skipped int
}

// BackfillReservationServicePersonnel fills reservation_services.personnel_id
// from the reservation's derived lead (reservations.personnel_id) wherever the
// line column is still NULL. It follows the BackfillServiceTreatments pattern:
//
//   - only NULL rows are considered, so a non-NULL line is NEVER overwritten
//     and a second run is a no-op;
//   - an orphan line (no matching reservation) or a failed update is logged and
//     skipped, never aborting startup;
//   - updates are issued per row with the "personnel_id IS NULL" guard in the
//     WHERE clause, so a concurrent write cannot be clobbered.
//
// It returns the stats so the outcome is measurable and testable.
func BackfillReservationServicePersonnel() ReservationPersonnelBackfillStats {
	var stats ReservationPersonnelBackfillStats

	var lines []models.ReservationService
	if err := DB.Where("personnel_id IS NULL").Find(&lines).Error; err != nil {
		utils.Log.Errorf("[SEED] Reservation service personnel backfill read failed: %v", err)
		return stats
	}
	stats.Eligible = len(lines)
	if stats.Eligible == 0 {
		utils.Log.Infof("[SEED] Reservation service personnel backfill: nothing to fill (idempotent no-op)")
		return stats
	}

	// Resolve each reservation's lead in one query instead of one per line.
	ids := make([]uint, 0, len(lines))
	seen := make(map[uint]struct{}, len(lines))
	for _, line := range lines {
		if _, ok := seen[line.ReservationID]; ok {
			continue
		}
		seen[line.ReservationID] = struct{}{}
		ids = append(ids, line.ReservationID)
	}
	var reservations []models.Reservation
	if err := DB.Select("id, personnel_id").Where("id IN ?", ids).Find(&reservations).Error; err != nil {
		utils.Log.Errorf("[SEED] Reservation service personnel backfill: could not load reservations: %v", err)
		return stats
	}
	lead := make(map[uint]uint, len(reservations))
	for _, r := range reservations {
		lead[r.ID] = r.PersonnelID
	}

	for _, line := range lines {
		personnelID, ok := lead[line.ReservationID]
		if !ok {
			stats.Skipped++
			utils.Log.Warnf("[SEED] Reservation service personnel backfill: skipping line %d, reservation %d not found", line.ID, line.ReservationID)
			continue
		}
		res := DB.Model(&models.ReservationService{}).
			Where("id = ? AND personnel_id IS NULL", line.ID).
			Update("personnel_id", personnelID)
		if res.Error != nil {
			stats.Skipped++
			utils.Log.Errorf("[SEED] Reservation service personnel backfill: failed to fill line %d: %v", line.ID, res.Error)
			continue
		}
		stats.Filled += int(res.RowsAffected)
	}

	utils.Log.Infof("[SEED] Reservation service personnel backfill: filled %d/%d unset lines (%d skipped)",
		stats.Filled, stats.Eligible, stats.Skipped)
	return stats
}
