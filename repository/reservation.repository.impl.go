package repository

import (
	"errors"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"gorm.io/gorm"
)

type ReservationRepositoryImpl struct {
	db *gorm.DB
}

func NewReservationRepositoryImpl(db *gorm.DB) ReservationRepository {
	return &ReservationRepositoryImpl{db: db}
}

// personnelMembershipSQL is the ONE definition of "this reservation involves
// this professional": the reservation's DERIVED LEAD, or any live service line
// attributed to them. It is the SQL twin of the agenda membership test and is
// consumed by FindByPersonnelId (the agenda), the write-path overlap checks,
// the status-transition guard (IsPersonnelMember) and the staff metrics
// aggregations — so "you can see it" and "you can operate on it" can never
// drift, and no path ever answers membership with a per-line query.
const personnelMembershipSQL = "(personnel_id = ? OR id IN (SELECT reservation_id FROM reservation_services WHERE personnel_id = ? AND deleted_at IS NULL))"

// personnelMembershipArgs returns the bind arguments for personnelMembershipSQL.
func personnelMembershipArgs(personnelID uint) []interface{} {
	return []interface{}{personnelID, personnelID}
}

func (r *ReservationRepositoryImpl) Create(reservation *models.Reservation) error {
	return r.db.Create(reservation).Error
}

func (r *ReservationRepositoryImpl) Update(reservation *models.Reservation) error {
	return r.db.Save(reservation).Error
}

func (r *ReservationRepositoryImpl) Delete(reservation *models.Reservation) error {
	return r.db.Delete(reservation).Error
}

func (r *ReservationRepositoryImpl) FindById(reservationID uint) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.
		Preload("ReservationServices.Service.Category").Preload("Client").Preload("Personnel").Preload("BeautyCenter").
		First(&reservation, reservationID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("reservation not found")
		}
		return nil, err
	}
	projectCategoryName(&reservation)
	return &reservation, nil
}

func (r *ReservationRepositoryImpl) FindUpcomingByClientID(clientID uint) (*models.Reservation, error) {
	var out []models.Reservation
	today := time.Now().Format("2006-01-02")
	// Find (unlike First) does not treat an empty result as an error, so a
	// client with no upcoming reservation is a normal empty state instead of
	// a spurious "record not found" error line.
	err := r.db.
		Preload("ReservationServices.Service.Category").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("client_id = ? AND date >= ? AND status NOT IN (?)", clientID, today, []string{"COMPLETED", "CANCELLED"}).
		Order("date ASC, start_time ASC").
		Limit(1).
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	projectCategoryNames(out)
	return &out[0], nil
}

func (r *ReservationRepositoryImpl) FindByClientID(clientID uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	err := r.db.
		Preload("ReservationServices.Service.Category").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("client_id = ?", clientID).
		Order("date DESC, start_time DESC").
		Find(&reservations).Error
	if err != nil {
		return reservations, err
	}
	projectCategoryNames(reservations)
	return reservations, nil
}

// projectCategoryName fills every service line's transient CategoryName from the
// preloaded Service.Category chain, so the model's own wire shape carries the
// flat `category_name` the client reservation list reads. It performs NO query:
// when the chain was not preloaded, Category.Name is empty and the key stays
// omitted (omitempty). Keeping the projection here — instead of a per-line
// lookup — is what avoids reintroducing an N+1 for a category label.
func projectCategoryName(reservation *models.Reservation) {
	for i := range reservation.ReservationServices {
		line := &reservation.ReservationServices[i]
		line.CategoryName = line.Service.Category.Name
	}
}

// projectCategoryNames is the slice form of projectCategoryName.
func projectCategoryNames(reservations []models.Reservation) {
	for i := range reservations {
		projectCategoryName(&reservations[i])
	}
}

func (r *ReservationRepositoryImpl) FindByBeautyCenterId(beautyCenterID uint, from, to *time.Time, clientID *uint) ([]models.Reservation, error) {
	var reservations []models.Reservation
	query := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("beauty_center_id = ?", beautyCenterID)

	if from != nil && to != nil {
		// Range-scoped agenda (design D4): inclusive range, chronological ordering.
		query = query.Where("date >= ? AND date <= ?", *from, *to).Order("start_time ASC")
	} else if from == nil && to == nil && clientID == nil {
		// No filters: return all reservations for this center (unchanged behavior).
		// The existing callers that only pass `date` will now pass nil, nil.
	} else if clientID != nil {
		// Client-scoped history (design D3): strict center+client filter with
		// reverse chronological order. Takes precedence over the date-agenda
		// ordering when both params are present.
		query = query.Where("client_id = ?", *clientID).Order("date DESC, start_time DESC")
	} else if from != nil && to == nil {
		// Single from-date: treat as exact day query.
		query = query.Where("date = ?", *from).Order("start_time ASC")
	} else if from == nil && to != nil {
		// Single to-date: treat as exact day query.
		query = query.Where("date = ?", *to).Order("start_time ASC")
	}

	err := query.Find(&reservations).Error
	return reservations, err
}

// FindByPersonnelId implements ReservationRepository. A professional's agenda
// must contain every reservation that has a service line attributed to them —
// not only the ones where they are the derived lead. The shared
// personnelMembershipSQL predicate keeps lead membership (and line-less legacy
// rows) working while the subquery adds the non-lead lines. The where clause is
// assembled conditionally so nil from/to stay open-ended while GORM still
// handles the bound parameters.
func (r *ReservationRepositoryImpl) FindByPersonnelId(personnelID uint, from, to *time.Time) ([]models.Reservation, error) {
	var reservations []models.Reservation
	query := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where(personnelMembershipSQL, personnelMembershipArgs(personnelID)...)
	if from != nil {
		query = query.Where("date >= ?", *from)
	}
	if to != nil {
		query = query.Where("date <= ?", *to)
	}
	err := query.Order("date ASC, start_time ASC").Find(&reservations).Error
	return reservations, err
}

// IsPersonnelMember reports whether personnelID is the reservation's DERIVED
// LEAD or owns one of its live service lines — the ONE membership definition
// shared with the agenda (personnelMembershipSQL). The status-transition guard
// uses it so a professional can operate on exactly what they can see. It is a
// single bounded EXISTS-style COUNT, never a query per line.
func (r *ReservationRepositoryImpl) IsPersonnelMember(reservationID, personnelID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.Reservation{}).
		Where("reservations.id = ?", reservationID).
		Where(personnelMembershipSQL, personnelMembershipArgs(personnelID)...).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// PersonnelStatusTotals aggregates the acting professional's reservation counts
// over the half-open range [from, to), scoped by the shared membership
// predicate: appointments excludes CANCELLED (mirroring the admin
// appointmentsSeries), completed/cancelled/no_show are the status buckets the
// staff metrics expose. ONE query, reservation-level (never joined to lines, so
// a shared booking is counted once per professional).
func (r *ReservationRepositoryImpl) PersonnelStatusTotals(personnelID uint, from, to time.Time) (PersonnelStatusTotalsRow, error) {
	var row PersonnelStatusTotalsRow
	err := r.db.Model(&models.Reservation{}).
		Where("date >= ? AND date < ?", from, to).
		Where(personnelMembershipSQL, personnelMembershipArgs(personnelID)...).
		Select(`COALESCE(SUM(CASE WHEN status <> 'CANCELLED' THEN 1 ELSE 0 END), 0) AS appointments,
			       COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0) AS completed,
			       COALESCE(SUM(CASE WHEN status = 'CANCELLED' THEN 1 ELSE 0 END), 0) AS cancelled,
			       COALESCE(SUM(CASE WHEN status = 'NO_SHOW' THEN 1 ELSE 0 END), 0) AS no_show`).
		Scan(&row).Error
	return row, err
}

// PersonnelWindowTotals returns the professional's status buckets for BOTH
// windows — the requested [splitFrom, to) and the immediately preceding
// [prevFrom, splitFrom) — in ONE statement, plus the centre currency via a
// scalar subquery. Folding the currency here (instead of a separate
// CenterCurrency round-trip) is deliberate: the staff metrics endpoint must
// stay at a CONSTANT number of statements as additive blocks land, and this is
// the one statement that always returns exactly one row even when both windows
// are empty. The CASE split is portable SQL (no date functions), so it behaves
// identically on Postgres and the SQLite test harness.
func (r *ReservationRepositoryImpl) PersonnelWindowTotals(personnelID, centerID uint, prevFrom, splitFrom, to time.Time) (PersonnelWindowTotalsRow, error) {
	var row PersonnelWindowTotalsRow
	err := r.db.Model(&models.Reservation{}).
		Where("date >= ? AND date < ?", prevFrom, to).
		Where(personnelMembershipSQL, personnelMembershipArgs(personnelID)...).
		Select(`COALESCE(SUM(CASE WHEN date >= ? AND status <> 'CANCELLED' THEN 1 ELSE 0 END), 0) AS appointments,
			       COALESCE(SUM(CASE WHEN date >= ? AND status = 'COMPLETED' THEN 1 ELSE 0 END), 0) AS completed,
			       COALESCE(SUM(CASE WHEN date >= ? AND status = 'CANCELLED' THEN 1 ELSE 0 END), 0) AS cancelled,
			       COALESCE(SUM(CASE WHEN date >= ? AND status = 'NO_SHOW' THEN 1 ELSE 0 END), 0) AS no_show,
			       COALESCE(SUM(CASE WHEN date < ? AND status <> 'CANCELLED' THEN 1 ELSE 0 END), 0) AS prev_appointments,
			       COALESCE(SUM(CASE WHEN date < ? AND status = 'COMPLETED' THEN 1 ELSE 0 END), 0) AS prev_completed,
			       COALESCE(SUM(CASE WHEN date < ? AND status = 'CANCELLED' THEN 1 ELSE 0 END), 0) AS prev_cancelled,
			       COALESCE(SUM(CASE WHEN date < ? AND status = 'NO_SHOW' THEN 1 ELSE 0 END), 0) AS prev_no_show,
			       COALESCE((SELECT currency FROM beauty_centers WHERE id = ? AND deleted_at IS NULL LIMIT 1), '') AS currency`,
			splitFrom, splitFrom, splitFrom, splitFrom,
			splitFrom, splitFrom, splitFrom, splitFrom,
			centerID).
		Scan(&row).Error
	return row, err
}

// personnelSegmentRow is one (reservation, service line) join row used to
// rebuild a reservation with its ordered lines in a single statement. The line
// columns are pointers: a legacy line-less reservation yields one row with NULL
// line columns and is handed to the kernel as a reservation with no lines, so
// its envelope fallback still blocks exactly as before.
type personnelSegmentRow struct {
	ReservationID uint      `gorm:"column:reservation_id"`
	LeadID        uint      `gorm:"column:lead_id"`
	Date          time.Time `gorm:"column:date"`
	StartTime     string    `gorm:"column:start_time"`
	EndTime       string    `gorm:"column:end_time"`
	LineID        *uint     `gorm:"column:line_id"`
	LineDuration  *int      `gorm:"column:line_duration"`
	LinePersonnel *uint     `gorm:"column:line_personnel"`
}

// PersonnelBookedReservations loads the professional's bookable reservations in
// [from, to) with their ordered lines, in ONE statement. The membership filter
// runs inside a derived table (the shared personnelMembershipSQL, unchanged and
// unqualified) so joining reservation_services afterwards cannot make its bare
// `personnel_id`/`id` ambiguous. Non-CANCELLED only — the same busy definition
// the availability read path uses — and only rows that involve this
// professional as derived lead or via a service line.
func (r *ReservationRepositoryImpl) PersonnelBookedReservations(personnelID, centerID uint, from, to time.Time) ([]models.Reservation, error) {
	var rows []personnelSegmentRow
	inner := "SELECT id, personnel_id, date, start_time, end_time, beauty_center_id, status " +
		"FROM reservations WHERE reservations.deleted_at IS NULL AND " + personnelMembershipSQL
	err := r.db.
		Table("("+inner+") AS r", personnelMembershipArgs(personnelID)...).
		Select(`r.id AS reservation_id, r.personnel_id AS lead_id, r.date AS date,
			       r.start_time AS start_time, r.end_time AS end_time,
			       reservation_services.id AS line_id,
			       reservation_services.duration AS line_duration,
			       reservation_services.personnel_id AS line_personnel`).
		Joins("LEFT JOIN reservation_services ON reservation_services.reservation_id = r.id AND reservation_services.deleted_at IS NULL").
		Where("r.beauty_center_id = ? AND r.status <> ? AND r.date >= ? AND r.date < ?", centerID, "CANCELLED", from, to).
		Order("r.id ASC, reservation_services.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]models.Reservation, 0, len(rows))
	for _, row := range rows {
		if len(out) == 0 || out[len(out)-1].ID != row.ReservationID {
			out = append(out, models.Reservation{
				ID:          row.ReservationID,
				PersonnelID: row.LeadID,
				Date:        row.Date,
				StartTime:   row.StartTime,
				EndTime:     row.EndTime,
			})
		}
		if row.LineID == nil {
			continue
		}
		line := models.ReservationService{
			ID:            *row.LineID,
			ReservationID: row.ReservationID,
			PersonnelID:   row.LinePersonnel,
		}
		if row.LineDuration != nil {
			line.Duration = *row.LineDuration
		}
		last := &out[len(out)-1]
		last.ReservationServices = append(last.ReservationServices, line)
	}
	return out, nil
}

// PersonnelRevenueByDay is the per-day twin of the admin RevenueByDay, but
// attributed PER LINE with the shared personnelRevenueExpr (the Phase 2b rule),
// scoped to the acting professional's lines. COMPLETED only. ONE query.
func (r *ReservationRepositoryImpl) PersonnelRevenueByDay(personnelID uint, from, to time.Time) ([]RevenueDayRow, error) {
	var rows []RevenueDayRow
	err := r.db.Model(&models.Reservation{}).
		Select(`reservations.date AS date, `+personnelRevenueExpr+` AS value`).
		Joins("LEFT JOIN reservation_services ON reservation_services.reservation_id = reservations.id AND reservation_services.deleted_at IS NULL").
		Joins("LEFT JOIN (SELECT reservation_id, SUM(price) AS line_total FROM reservation_services WHERE deleted_at IS NULL GROUP BY reservation_id) AS line_totals ON line_totals.reservation_id = reservations.id").
		Where("reservations.status = ? AND reservations.date >= ? AND reservations.date < ? AND COALESCE(reservation_services.personnel_id, reservations.personnel_id) = ?",
			"COMPLETED", from, to, personnelID).
		Group("reservations.date").
		Order("reservations.date ASC").
		Scan(&rows).Error
	return rows, err
}

// PersonnelAppointmentsByDay counts the acting professional's non-CANCELLED
// reservations per day in [from, to), grouped by date. It mirrors the admin
// AppointmentsByDay definition but scopes by membership. ONE query; COUNT(*)
// counts each booking once (no line join, so a shared booking never inflates).
func (r *ReservationRepositoryImpl) PersonnelAppointmentsByDay(personnelID uint, from, to time.Time) ([]CountDayRow, error) {
	var rows []CountDayRow
	err := r.db.Model(&models.Reservation{}).
		Where("status <> ? AND date >= ? AND date < ?", "CANCELLED", from, to).
		Where(personnelMembershipSQL, personnelMembershipArgs(personnelID)...).
		Select("date, COUNT(*) AS value").
		Group("date").
		Order("date ASC").
		Scan(&rows).Error
	return rows, err
}

// PersonnelTopServices mirrors TopServicesByRevenue (revenue DESC, then
// count DESC, then service_id ASC; same limit) but scoped to the acting
// professional's lines: revenue is SUM(price) and count is
// COUNT(DISTINCT reservation_id) over COMPLETED reservations in [from, to).
// ONE query.
func (r *ReservationRepositoryImpl) PersonnelTopServices(personnelID uint, from, to time.Time, limit int) ([]TopServiceRow, error) {
	var rows []TopServiceRow
	err := r.db.Model(&models.ReservationService{}).
		Select(`reservation_services.service_id,
			       reservation_services.service_name,
			       SUM(reservation_services.price) AS revenue,
			       COUNT(DISTINCT reservation_services.reservation_id) AS appointment_count`).
		Joins("JOIN reservations ON reservations.id = reservation_services.reservation_id AND reservations.deleted_at IS NULL").
		Where("reservations.status = ? AND reservations.date >= ? AND reservations.date < ? AND COALESCE(reservation_services.personnel_id, reservations.personnel_id) = ?",
			"COMPLETED", from, to, personnelID).
		Group("reservation_services.service_id, reservation_services.service_name").
		Order("revenue DESC, appointment_count DESC, reservation_services.service_id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// CenterCurrency reads just the centre's ISO currency. One single-column
// statement (no eager-loads); a missing/soft-deleted centre yields "".
func (r *ReservationRepositoryImpl) CenterCurrency(centerID uint) (string, error) {
	var currency string
	err := r.db.Model(&models.BeautyCenter{}).
		Where("id = ?", centerID).
		Pluck("currency", &currency).Error
	return currency, err
}

func (r *ReservationRepositoryImpl) FindAll(page, limit int, centerID uint) ([]models.Reservation, int64, error) {
	var reservations []models.Reservation
	var total int64

	base := r.db.Model(&models.Reservation{}).Where("beauty_center_id = ?", centerID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("beauty_center_id = ?", centerID).
		Offset(offset).
		Limit(limit).
		Find(&reservations).Error
	return reservations, total, err
}

// CheckOverlap reports whether personnelID is busy inside [startTime, endTime)
// on date at the center. A professional is busy only for THEIR OWN segments of
// a reservation, not for the whole envelope of a shared reservation.
//
// Strategy (superset SQL prefilter + exact Go refinement):
//
//  1. SQL narrows to envelope-overlapping, non-cancelled reservations at the
//     center (exactly the historical predicate). For a professional it also
//     requires the reservation to carry them — either as its derived lead or
//     as the professional of one of its service lines. This is a SUPERSET of
//     the true conflicts: any segment that overlaps the window lies inside its
//     reservation's envelope, so no real conflict can be filtered out.
//  2. Go derives each candidate's segments (models.DeriveReservationSegments)
//     and checks overlap against the professional's own segments, which can
//     only narrow the set.
//
// For a single-professional reservation the segments tile the envelope
// contiguously, so step 2 returns the same answer the envelope check always
// did — a byte-for-byte no-op for all existing data.
//
// personnelID == 0 keeps the historical center-wide semantics: any
// envelope-overlapping reservation blocks.
func (r *ReservationRepositoryImpl) CheckOverlap(centerID uint, date time.Time, startTime, endTime string, personnelID, excludeID uint) (bool, error) {
	query := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND date = ? AND status != 'CANCELLED'", centerID, date).
		Where("((start_time < ? AND end_time > ?) OR (start_time >= ? AND start_time < ?) OR (end_time > ? AND end_time <= ?))",
			endTime, startTime, startTime, endTime, startTime, endTime)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	if personnelID == 0 {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	}

	var candidates []models.Reservation
	err := query.
		Where(personnelMembershipSQL, personnelMembershipArgs(personnelID)...).
		Preload("ReservationServices", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Find(&candidates).Error
	if err != nil {
		return false, err
	}

	startMinutes := models.ParseClockMinutes(startTime)
	endMinutes := models.ParseClockMinutes(endTime)
	for i := range candidates {
		if models.ReservationBusyForPersonnel(&candidates[i], personnelID, startMinutes, endMinutes) {
			return true, nil
		}
	}
	return false, nil
}

// CheckOverlapSegments is the multi-professional twin of CheckOverlap: it
// reports whether ANY existing non-cancelled reservation has, for the SAME
// professional, a segment overlapping one of the prospective segments.
//
// It mirrors CheckOverlap's two-step strategy so the write path stays one
// definition of "a professional is busy when":
//
//  1. ONE SQL pass narrows to envelope-overlapping, non-cancelled reservations
//     at the center (the [startTime, endTime) envelope of the NEW reservation
//     is the union of its segments, so it is a SUPERSET of every true
//     conflict — no real conflict is filtered out).
//  2. Go derives each candidate's segments and tests the professional's own
//     segments with models.ReservationBusyForPersonnel.
//
// Unlike CheckOverlap it is NOT scoped to one professional: the caller passes
// the whole segment list (bundle), so a plan may not double-book ANY of its
// professionals. Candidates are loaded with their service lines in one preload
// — no query per professional or per segment.
func (r *ReservationRepositoryImpl) CheckOverlapSegments(centerID uint, date time.Time, startTime, endTime string, segments []models.Segment, excludeID uint) (bool, error) {
	if len(segments) == 0 {
		return false, nil
	}

	query := r.db.Model(&models.Reservation{}).
		Where("beauty_center_id = ? AND date = ? AND status != 'CANCELLED'", centerID, date).
		Where("((start_time < ? AND end_time > ?) OR (start_time >= ? AND start_time < ?) OR (end_time > ? AND end_time <= ?))",
			endTime, startTime, startTime, endTime, startTime, endTime)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	var candidates []models.Reservation
	err := query.
		Preload("ReservationServices", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Find(&candidates).Error
	if err != nil {
		return false, err
	}

	for i := range candidates {
		for _, seg := range segments {
			if models.ReservationBusyForPersonnel(&candidates[i], seg.PersonnelID, seg.StartMinutes, seg.EndMinutes) {
				return true, nil
			}
		}
	}
	return false, nil
}

// UpdatePersonnelID sets the reservation's professional: the DERIVED LEAD and
// every one of its service lines, in one transaction, so the two can never
// diverge. Until per-line assignment exists, reassigning a reservation means
// reassigning all of it - and the line-level attribution we now expose would
// otherwise keep naming the previous professional.
func (r *ReservationRepositoryImpl) UpdatePersonnelID(reservationID uint, personnelID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Reservation{}).
			Where("id = ?", reservationID).
			Update("personnel_id", personnelID).Error; err != nil {
			return err
		}
		return tx.Model(&models.ReservationService{}).
			Where("reservation_id = ?", reservationID).
			Update("personnel_id", personnelID).Error
	})
}

func (r *ReservationRepositoryImpl) DeleteReservationServices(reservationID uint) error {
	return r.db.Where("reservation_id = ?", reservationID).Delete(&models.ReservationService{}).Error
}

// FindExpiredHolds implements ReservationRepository. Only live holds
// (PENDING/CONFIRMED) qualify — terminal rows are never rewritten.
func (r *ReservationRepositoryImpl) FindExpiredHolds(now time.Time) ([]models.Reservation, error) {
	var out []models.Reservation
	err := r.db.Where("payment_status = ?", "PENDING_PROOF").
		Where("status IN ?", []string{"PENDING", "CONFIRMED"}).
		Where("hold_expires_at IS NOT NULL AND hold_expires_at < ?", now).
		Find(&out).Error
	return out, err
}

// FindConfirmedInDateRange implements ReservationRepository. The upper
// bound is exclusive next-day-start ("date < to+1d") instead of "<= to":
// SQLite compares datetimes lexicographically, so a same-day row stored as
// "2026-09-22 00:00:00+00:00" would fail "<= 2026-09-22". Both Postgres
// (date type) and SQLite (string compare) evaluate the exclusive bound
// correctly.
func (r *ReservationRepositoryImpl) FindConfirmedInDateRange(from, to time.Time) ([]models.Reservation, error) {
	var out []models.Reservation
	err := r.db.Where("status = ?", "CONFIRMED").
		Where("date >= ? AND date < ?", from.Format("2006-01-02"), to.Add(24*time.Hour).Format("2006-01-02")).
		Find(&out).Error
	return out, err
}

func (r *ReservationRepositoryImpl) FindByToken(token string) (*models.Reservation, error) {
	var reservation models.Reservation
	err := r.db.
		Preload("ReservationServices").Preload("BeautyCenter").Preload("Client").Preload("Personnel").
		Where("verification_token = ?", token).
		First(&reservation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &reservation, nil
}

// CompleteVerification performs the atomic exactly-once update: only a
// reservation that is still PENDING or CONFIRMED is completed, the token and
// expiry are cleared, and the status is persisted in canonical UPPERCASE.
func (r *ReservationRepositoryImpl) CompleteVerification(reservation *models.Reservation) (bool, error) {
	result := r.db.Model(&models.Reservation{}).
		Where("id = ? AND status IN ?", reservation.ID, []string{"PENDING", "CONFIRMED"}).
		Updates(map[string]interface{}{
			"status":             "COMPLETED",
			"verification_token": gorm.Expr("NULL"),
			"token_expires_at":   gorm.Expr("NULL"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// Transact exposes the repository's database transaction so the QR
// verification service can compose completion + token debit atomically
// (feat-token-economy-config D4).
func (r *ReservationRepositoryImpl) Transact(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// CompleteVerificationTx is the tx-aware variant of CompleteVerification:
// same conditional update (PENDING/CONFIRMED → COMPLETED, token cleared),
// executed inside the caller's transaction. RowsAffected zero means another
// scan already completed it.
func (r *ReservationRepositoryImpl) CompleteVerificationTx(tx *gorm.DB, reservationID uint) (bool, error) {
	result := tx.Model(&models.Reservation{}).
		Where("id = ? AND status IN ?", reservationID, []string{"PENDING", "CONFIRMED"}).
		Updates(map[string]interface{}{
			"status":             "COMPLETED",
			"verification_token": gorm.Expr("NULL"),
			"token_expires_at":   gorm.Expr("NULL"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
