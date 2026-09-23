package configs

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
)

func SeedDefaultMissions() {
	missions := []models.TokenMission{
		{
			Name:         "First Completed Reservation",
			Description:  "Complete your first reservation",
			TriggerType:  "reservation_completed",
			TriggerCount: 1,
			RewardTokens: 10000,
			Icon:         "🎯",
			IsActive:     true,
			SortOrder:    1,
		},
		{
			Name:         "Loyalty Achiever",
			Description:  "Complete 50 reservations in a month",
			TriggerType:  "reservation_completed",
			TriggerCount: 50,
			RewardTokens: 25000,
			Icon:         "🏆",
			IsActive:     true,
			SortOrder:    2,
		},
		{
			Name:         "Stamp Promoter",
			Description:  "Activate a stamp card promotion",
			TriggerType:  "stamp_activated",
			TriggerCount: 1,
			RewardTokens: 15000,
			Icon:         "🎟️",
			IsActive:     true,
			SortOrder:    3,
		},
		{
			Name:         "Profile Star",
			Description:  "Complete your profile with photo and description",
			TriggerType:  "profile_completed",
			TriggerCount: 1,
			RewardTokens: 5000,
			Icon:         "⭐",
			IsActive:     true,
			SortOrder:    4,
		},
		{
			Name:         "Receive 5 Reviews",
			Description:  "Receive 5 reviews from your clients",
			TriggerType:  "review_received",
			TriggerCount: 5,
			RewardTokens: 3000,
			Icon:         "💬",
			IsActive:     true,
			SortOrder:    5,
		},
		{
			Name:         "10 Completed Reservations",
			Description:  "Complete 10 reservations",
			TriggerType:  "reservation_completed",
			TriggerCount: 10,
			RewardTokens: 6000,
			Icon:         "🥉",
			IsActive:     true,
			SortOrder:    6,
		},
		{
			Name:         "Refer a Colleague",
			Description:  "Refer a colleague whose center gets verified",
			TriggerType:  "referral",
			TriggerCount: 1,
			RewardTokens: 30000,
			Icon:         "🤝",
			IsActive:     true,
			SortOrder:    7,
		},
	}

	// feat-token-missions D3: review responses don't exist as a product
	// surface yet — the legacy trigger type is deactivated so stale rows on
	// pre-existing databases never render or progress.
	if err := DB.Model(&models.TokenMission{}).
		Where("trigger_type = ?", "review_responded").
		Update("is_active", false).Error; err != nil {
		utils.Log.Errorf("[SEED] Failed to deactivate legacy review_responded mission: %v", err)
	}

	for _, mission := range missions {
		var existing models.TokenMission
		result := DB.Where("trigger_type = ? AND trigger_count = ?", mission.TriggerType, mission.TriggerCount).First(&existing)
		if result.Error != nil {
			if err := DB.Create(&mission).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create mission %s: %v", mission.Name, err)
			} else {
				utils.Log.Infof("[SEED] Created mission: %s (+%.0f YOTTUs)", mission.Name, utils.FromMilli(mission.RewardTokens))
			}
		} else if existing.Name != mission.Name ||
			existing.Description != mission.Description ||
			existing.Icon != mission.Icon ||
			existing.SortOrder != mission.SortOrder ||
			existing.RewardTokens != mission.RewardTokens ||
			!existing.IsActive {
			// Self-heal drifted legacy rows (e.g. reward loaded as whole units
			// instead of milli-YOTTU) back to the canonical definition.
			if err := DB.Model(&existing).Updates(map[string]interface{}{
				"name":          mission.Name,
				"description":   mission.Description,
				"icon":          mission.Icon,
				"sort_order":    mission.SortOrder,
				"reward_tokens": mission.RewardTokens,
				"is_active":     true,
			}).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to heal mission %s: %v", mission.Name, err)
			} else {
				utils.Log.Infof("[SEED] Healed drifted mission %s → %.0f YOTTUs", mission.Name, utils.FromMilli(mission.RewardTokens))
			}
		}
	}
}

// SeedMarketConfigs seeds the per-currency token economy configuration
// (feat-token-economy-config) with the same create-or-heal pattern as
// missions: missing rows are created, drifted rows are restored to canonical.
func SeedMarketConfigs() {
	configs := []models.MarketConfig{
		// Bolivia — launch market. 1 YOTTU = 1 BOB; commission 3.5%;
		// floor 0.35 YOTTU (= commission of a 10 BOB reference ticket).
		{
			Currency:        "BOB",
			YottuToFiatRate: 1.0,
			CommissionPct:   3.5,
			MinChargeMilli:  350,
		},
	}

	for _, cfg := range configs {
		var existing models.MarketConfig
		result := DB.Where("currency = ?", cfg.Currency).First(&existing)
		if result.Error != nil {
			if err := DB.Create(&cfg).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create market config %s: %v", cfg.Currency, err)
			} else {
				utils.Log.Infof("[SEED] Created market config %s: %.2f%% / floor %.2f YOTTUs",
					cfg.Currency, cfg.CommissionPct, utils.FromMilli(cfg.MinChargeMilli))
			}
		} else if existing.YottuToFiatRate != cfg.YottuToFiatRate ||
			existing.CommissionPct != cfg.CommissionPct ||
			existing.MinChargeMilli != cfg.MinChargeMilli {
			if err := DB.Model(&existing).Updates(map[string]interface{}{
				"yottu_to_fiat_rate": cfg.YottuToFiatRate,
				"commission_pct":     cfg.CommissionPct,
				"min_charge_milli":   cfg.MinChargeMilli,
			}).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to heal market config %s: %v", cfg.Currency, err)
			} else {
				utils.Log.Infof("[SEED] Healed drifted market config %s → %.2f%% / floor %.2f",
					cfg.Currency, cfg.CommissionPct, utils.FromMilli(cfg.MinChargeMilli))
			}
		}
	}
}

// BackfillLegacyVideos migrates legacy `videos` rows into feed_posts and then
// drops the legacy table (feat-video-feed-unification D1/D7). No-op when the
// legacy table is absent. Invalid rows are log-and-skip — the batch never
// aborts. PR body documents DB-backup expectation for rollback.
func BackfillLegacyVideos() {
	if !DB.Migrator().HasTable("videos") {
		return
	}

	type legacyRow struct {
		BeautyCenterID uint
		Title          string
		Description    string
		FilePath       string
		ThumbnailURL   string
		Views          int
	}

	var legacy []legacyRow
	if err := DB.Table("videos").
		Select("beauty_center_id, title, description, file_path, thumbnail_url, views").
		Where("deleted_at IS NULL").
		Scan(&legacy).Error; err != nil {
		utils.Log.Errorf("[SEED] legacy videos backfill read failed: %v", err)
		return
	}
	if len(legacy) == 0 {
		utils.Log.Infof("[SEED] legacy videos table empty — nothing to backfill")
		return
	}

	migrated := 0
	for _, row := range legacy {
		if row.FilePath == "" || row.BeautyCenterID == 0 {
			utils.Log.Warnf("[SEED] skipping invalid legacy video (center %d): %q", row.BeautyCenterID, row.Title)
			continue
		}
		caption := row.Title
		if len(caption) < 3 && len(row.Description) <= 255 {
			caption = row.Description
		}
		post := models.FeedPost{
			BeautyCenterID: row.BeautyCenterID,
			VideoURL:       row.FilePath,
			ThumbnailURL:   row.ThumbnailURL,
			Caption:        caption,
			ViewsCount:     row.Views,
			IsActive:       true,
		}
		if err := DB.Create(&post).Error; err != nil {
			utils.Log.Errorf("[SEED] failed to migrate legacy video %q: %v", row.Title, err)
			continue
		}
		migrated++
	}

	if err := DB.Migrator().DropTable("videos"); err != nil {
		utils.Log.Errorf("[SEED] backfill complete but failed to drop legacy videos table: %v", err)
		return
	}
	utils.Log.Infof("[SEED] Backfilled %d/%d legacy videos into feed_posts; legacy table dropped", migrated, len(legacy))
}

// ── Demo service catalog (realistic Bolivian salon, prices in BOB) ──
type demoServiceDef struct {
	Name        string
	Description string
	Duration    int
	Price       float64
}

type demoCategoryDef struct {
	Name     string
	Icon     string
	Services []demoServiceDef
}

var demoCatalog = []demoCategoryDef{
	{
		Name: "Peluquería", Icon: "💇",
		Services: []demoServiceDef{
			{"Corte clásico", "Corte de cabello con lavado y peinado final.", 45, 60},
			{"Corte + barba", "Corte completo más arreglo y perfilado de barba.", 60, 90},
			{"Tinte completo", "Coloración completa con productos profesionales.", 120, 250},
			{"Balayage", "Mechas balayage con matización y tratamiento.", 150, 350},
		},
	},
	{
		Name: "Manicuría", Icon: "💅",
		Services: []demoServiceDef{
			{"Manicuría semipermanente", "Esmaltado semipermanente con diseño base.", 60, 80},
			{"Uñas acrílicas", "Set completo de uñas acrílicas esculpidas.", 90, 150},
			{"Nail art", "Decoración personalizada por uña.", 30, 50},
		},
	},
	{
		Name: "Maquillaje", Icon: "💄",
		Services: []demoServiceDef{
			{"Maquillaje social", "Maquillaje para eventos con pestañas incluidas.", 60, 120},
			{"Maquillaje de novia", "Prueba + maquillaje del día con retoques.", 120, 350},
		},
	},
	{
		Name: "Estética facial", Icon: "✨",
		Services: []demoServiceDef{
			{"Limpieza facial", "Limpieza profunda con hidratación final.", 60, 150},
			{"Depilación de piernas", "Depilación completa con cera tibia.", 45, 100},
			{"Masaje relajante", "Masaje corporal de relajación con aceites.", 60, 180},
		},
	},
	{
		Name: "Barbería", Icon: "💈",
		Services: []demoServiceDef{
			{"Corte degradado", "Fade con navaja y acabado a detalle.", 40, 70},
			{"Afeitado a navaja", "Afeitado clásico con toalla caliente.", 30, 60},
		},
	},
}

// seedCenterServices creates categories + services for one demo center.
// Rotation by index keeps chips varied across centers: all get Peluquería
// + Manicuría, plus one rotating category (Estética / Maquillaje / Barbería).
func seedCenterServices(centerID uint, index int) int {
	total := 0
	cats := []demoCategoryDef{demoCatalog[0], demoCatalog[1], demoCatalog[2+(index%3)]}
	for _, c := range cats {
		category := models.Category{
			BeautyCenterID: centerID,
			Name:           c.Name,
			Description:    fmt.Sprintf("Servicios de %s.", c.Name),
			Icon:           c.Icon,
		}
		if err := DB.Create(&category).Error; err != nil {
			utils.Log.Errorf("[SEED] Failed to create category %q: %v", c.Name, err)
			continue
		}
		// Higher-index centers get deeper menus (2-4 services per category)
		n := 2 + (index % 2)
		if n > len(c.Services) {
			n = len(c.Services)
		}
		for _, s := range c.Services[:n] {
			service := models.Service{
				BeautyCenterID: centerID,
				CategoryID:     category.ID,
				Name:           s.Name,
				Description:    s.Description,
				Duration:       s.Duration,
				Price:          s.Price,
				IsActive:       true,
			}
			if err := DB.Create(&service).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create service %q: %v", s.Name, err)
				continue
			}
			total++
		}
	}
	return total
}

// SeedDemoCenters creates realistic demo beauty centers in Oruro, Bolivia
// when SEED_DEMO_CENTERS=true. Idempotent — skips if centers already exist.
// Only intended for development environments.
func SeedDemoCenters() {
	if os.Getenv("SEED_DEMO_CENTERS") != "true" {
		return
	}

	// Check if demo centers already exist (count by referral code prefix to avoid
	// LIKE escaping issues with special chars in email addresses).
	var count int64
	DB.Model(&models.BeautyCenter{}).Where("referral_code LIKE 'DEMO%'").Count(&count)
	if count >= 16 {
		// Backfill services for existing demo centers that lack them
		var ids []uint
		DB.Model(&models.BeautyCenter{}).Where("referral_code LIKE 'DEMO%'").Pluck("id", &ids)
		backfilled := 0
		for idx, id := range ids {
			var catCount int64
			DB.Model(&models.Category{}).Where("beauty_center_id = ?", id).Count(&catCount)
			if catCount == 0 {
				backfilled += seedCenterServices(id, idx)
			}
		}
		if backfilled > 0 {
			utils.Log.Infof("[SEED] Backfilled %d demo services", backfilled)
		} else {
			utils.Log.Infof("[SEED] Demo centers already exist (%d), skipping", count)
		}
		// Backfill deposit settings on the first 3 demo centers so the seña
		// flow is testable. No QR seeded: a fake QR would misdirect real
		// money, so the sheet falls back to coordinate-off-app text.
		var demoIDs []uint
		DB.Model(&models.BeautyCenter{}).Where("referral_code LIKE 'DEMO%'").Order("id").Limit(3).Pluck("id", &demoIDs)
		for _, id := range demoIDs {
			DB.Model(&models.BeautyCenter{}).Where("id = ? AND deposit_required = ?", id, false).Updates(map[string]interface{}{
				"deposit_required": true, "deposit_percent": 30,
				"deposit_trust_threshold": 70, "deposit_hold_minutes": 30,
			})
		}
		return
	}

	utils.Log.Infof("[SEED] Seeding demo beauty centers for Oruro, Bolivia...")

	rng := rand.New(rand.NewSource(42)) // deterministic for reproducibility
	hashedPwd, _ := utils.HashPassword("Demo1234!")
	now := time.Now()

	// ── Create demo clients ────────────────────────
	var existingClients int64
	DB.Model(&models.Client{}).Where("username LIKE 'demo_%'").Count(&existingClients)
	var demoClientIDs []uint

	if existingClients == 0 {
		clientNames := []struct{ First, Last, User string }{
			{"María", "González", "maria_g"},
			{"Juan", "Quispe", "juan_q"},
			{"Ana", "Mamani", "ana_m"},
			{"Carlos", "Flores", "carlos_f"},
			{"Laura", "Torres", "laura_t"},
			{"Pedro", "Vargas", "pedro_v"},
			{"Sofía", "Ramos", "sofia_r"},
			{"Diego", "Castro", "diego_c"},
			{"Valentina", "Herrera", "valentina_h"},
			{"Luis", "Mendoza", "luis_m"},
			{"Camila", "Silva", "camila_s"},
			{"Andrés", "Reyes", "andres_r"},
			{"Isabella", "Cruz", "isabella_c"},
			{"Fernando", "Ortega", "fernando_o"},
			{"Gabriela", "Morales", "gabriela_m"},
		}

		for i, c := range clientNames {
			client := models.Client{
				FirstName: c.First,
				LastName:  c.Last,
				Username:  "demo_" + c.User,
				Phone:     fmt.Sprintf("591-7%d%04d", i%10, 1000+i),
				Email:     fmt.Sprintf("demo_%s@demo.yottu.bo", c.User),
				Password:  hashedPwd,
				IsActive:  true,
			}
			if err := DB.Create(&client).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create demo client %s: %v", c.First, err)
				continue
			}
			demoClientIDs = append(demoClientIDs, client.ID)
		}
		utils.Log.Infof("[SEED] Created %d demo clients", len(demoClientIDs))
	} else {
		// Use existing demo clients
		var clients []models.Client
		DB.Where("username LIKE 'demo_%'").Find(&clients)
		for _, c := range clients {
			demoClientIDs = append(demoClientIDs, c.ID)
		}
	}

	if len(demoClientIDs) == 0 {
		utils.Log.Errorf("[SEED] No demo clients available, cannot seed centers")
		return
	}

	// ── Center definitions ─────────────────────────
	type centerDef struct {
		Name         string
		Address      string
		Neighborhood string
		Lat          float64
		Lng          float64
		Rating       float64 // 3.5 – 5.0
		ReviewCount  int
		ResCount     int
		PostCount    int
		FavCount     int
	}

	centers := []centerDef{
		// ── Centro Histórico (4) ─────────────────────
		{"Salón Belleza Real", "Av. 6 de Octubre #1234, entre Bolívar y Colón", "Centro", -17.9645, -67.1023, 4.8, 42, 85, 6, 28},
		{"Studio Hair Design", "Calle Bolívar #567, frente a la Catedral", "Centro", -17.9660, -67.0998, 4.5, 31, 62, 4, 19},
		{"Glamour Spa & Beauty", "Av. El Campesino #890, Local 3", "Centro", -17.9672, -67.1035, 4.2, 18, 45, 3, 12},
		{"Kami Beauty Studio", "Calle 6 de Octubre #1500, piso 2", "Centro", -17.9638, -67.1010, 4.9, 56, 110, 8, 35},

		// ── Ciudadela / Zona Norte (4) ────────────────
		{"Optimal Hair Studio", "Av. Juan Lechín Oquendo #2345, Zona Norte", "Ciudadela", -17.9580, -67.1080, 4.3, 22, 38, 3, 8},
		{"Maribel's Beauty Salon", "Calle Fortín Boquerón #678, Barrio Minero", "Ciudadela", -17.9565, -67.1055, 4.0, 15, 28, 2, 6},
		{"Glow Up Oruro", "Av. Ferroviaria #1122, cerca al mercado", "Ciudadela", -17.9590, -67.1095, 3.8, 9, 20, 2, 4},
		{"Centro de Estética Victoria", "Calle Comercio #456, Ciudadela", "Ciudadela", -17.9575, -67.1070, 4.6, 28, 55, 5, 16},

		// ── San Juan / Zona Sur (4) ──────────────────
		{"Bella Vista Spa", "Av. Sánchez de Lozada #3210, San Juan", "San Juan", -17.9720, -67.0950, 4.7, 35, 72, 5, 22},
		{"Style & Color Studio", "Calle Warnes #890, esq. Linares", "San Juan", -17.9735, -67.0975, 4.1, 14, 30, 3, 7},
		{"Nails & Beauty Oruro", "Av. Villazón #1567, Local 5", "San Juan", -17.9710, -67.0935, 3.9, 11, 22, 2, 5},
		{"Tu Look Oruro", "Calle Mercado #234, frente al parque", "San Juan", -17.9740, -67.0960, 4.4, 20, 42, 4, 11},

		// ── Zona Este / Dispersion (4) ───────────────
		{"Peluquería Los Andes", "Av. Circunvalación #4567, zona este", "Zona Este", -17.9620, -67.0880, 3.7, 7, 15, 1, 3},
		{"Beauty Corner", "Calle Sucre #1890, barrio ferroviario", "Ferroviario", -17.9605, -67.0910, 4.0, 12, 25, 2, 6},
		{"Espacio Belleza Urmiri", "Av. Tacna #2678, cerca al estadio", "Urmiri", -17.9695, -67.1120, 4.5, 25, 48, 4, 14},
		{"Hair Lab Oruro", "Calle Víctor Paz Estenssoro #3456, zona sur", "Zona Sur", -17.9760, -67.1000, 4.2, 16, 35, 3, 9},
	}

	reviewComments := []string{
		"Muy buen servicio, recomendado!",
		"Excelente atención, volveré seguro.",
		"Buen trabajo, pero un poco lento.",
		"La mejor peluquería de Oruro.",
		"Profesionales y amables.",
		"Me encantó el resultado.",
		"Regular, esperaba más calidad.",
		"Fantástico lugar, muy limpio.",
		"Precios justos y buen trato.",
		"Quedé conforme con el corte.",
		"Siempre vuelvo, nunca me fallan.",
		"Instalaciones modernas y cómodas.",
	}

	statuses := []string{"COMPLETED", "COMPLETED", "COMPLETED", "PENDING", "CANCELLED"}

	for i, def := range centers {
		// ── Center ───────────────────────────────────
		center := models.BeautyCenter{
			Name:         def.Name,
			Address:      fmt.Sprintf("%s, %s, Oruro", def.Address, def.Neighborhood),
			Email:        fmt.Sprintf("%s@demo.yottu.bo", slugify(def.Name)),
			Phone:        fmt.Sprintf("591-4-%d", 5000000+i),
			Url:          fmt.Sprintf("https://yottu.bo/center/%d", i+1),
			Description:  fmt.Sprintf("Salón de belleza en %s, Oruro. Servicios de peluquería, manicuría y estética.", def.Neighborhood),
			Latitude:     def.Lat,
			Longitude:    def.Lng,
			Password:     hashedPwd,
			Currency:     "BOB",
			IsActive:     true,
			ReferralCode: fmt.Sprintf("DEMO%03d", i+1),
		}
		// First 3 demo centers require seña so the deposit flow is testable.
		if i < 3 {
			center.DepositRequired = true
			center.DepositPercent = 30
			center.DepositTrustThreshold = 70
			center.DepositHoldMinutes = 30
		}

		if err := DB.Create(&center).Error; err != nil {
			utils.Log.Errorf("[SEED] Failed to create demo center %q: %v", def.Name, err)
			continue
		}
		// Use the auto-assigned ID from the database
		centerID := center.ID

		// ── Personnel (2 per center) ─────────────────
		var personnelIDs []uint
		for p := 0; p < 2; p++ {
			personnel := models.Personnel{
				BeautyCenterID: centerID,
				FirstName:      []string{"Carlos", "María"}[p],
				LastName:       []string{"Mamani", "Condori"}[p],
				Email:          fmt.Sprintf("staff%d_%d@demo.yottu.bo", centerID, p+1),
				Phone:          fmt.Sprintf("591-7-%d", 60000+int(centerID)*10+p),
				Password:       hashedPwd,
				IsActive:       true,
			}
			if err := DB.Create(&personnel).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create personnel for %s: %v", def.Name, err)
				continue
			}
			personnelIDs = append(personnelIDs, personnel.ID)
		}

		// ── Images ─────────────────────────────────
		DB.Create(&models.BeautyCenterImage{
			BeautyCenterID: centerID,
			ImageUrl:       fmt.Sprintf("https://picsum.photos/seed/%s/400/400", slugify(def.Name)),
			IsProfile:      true,
			Position:       0,
		})
		DB.Create(&models.BeautyCenterImage{
			BeautyCenterID: centerID,
			ImageUrl:       fmt.Sprintf("https://picsum.photos/seed/%s-cover/800/400", slugify(def.Name)),
			IsCover:        true,
			Position:       1,
		})

		// ── Working Hours (Mon-Sat) ────────────────
		for day := 1; day <= 6; day++ {
			startH := 9
			endH := 18
			if day == 6 {
				endH = 15 // Saturday shorter
			}
			if def.Lat < -17.97 { // south zone opens later
				startH = 10
			}
			DB.Create(&models.WorkingHour{
				BeautyCenterID: centerID,
				DayOfWeek:      day,
				StartTime:      fmt.Sprintf("%02d:00", startH),
				EndTime:        fmt.Sprintf("%02d:00", endH),
			})
		}

		// ── Categories + Services ────────────────
		svcCount := seedCenterServices(centerID, i)

		// ── Reviews (use real client IDs) ───────────
		for r := 0; r < def.ReviewCount; r++ {
			clientID := demoClientIDs[rng.Intn(len(demoClientIDs))]
			rating := clampFloat(def.Rating+float64(rng.Intn(3)-1), 1, 5)
			daysAgo := rng.Intn(90) + 1
			review := models.Review{
				BeautyCenterID: centerID,
				ClientID:       clientID,
				Rating:         int(rating),
				Comment:        reviewComments[rng.Intn(len(reviewComments))],
			}
			review.CreatedAt = now.AddDate(0, 0, -daysAgo)
			review.UpdatedAt = review.CreatedAt
			DB.Create(&review)
		}

		// ── Reservations (use real client + personnel IDs) ──
		for res := 0; res < def.ResCount; res++ {
			clientID := demoClientIDs[rng.Intn(len(demoClientIDs))]
			var personnelID uint
			if len(personnelIDs) > 0 {
				personnelID = personnelIDs[rng.Intn(len(personnelIDs))]
			} else {
				personnelID = 1 // fallback
			}
			daysAgo := rng.Intn(60) + 1
			hour := 9 + rng.Intn(9)
			status := statuses[rng.Intn(len(statuses))]

			reservation := models.Reservation{
				BeautyCenterID: centerID,
				ClientID:       clientID,
				PersonnelID:    personnelID,
				Date:           now.AddDate(0, 0, -daysAgo),
				StartTime:      fmt.Sprintf("%02d:00", hour),
				EndTime:        fmt.Sprintf("%02d:30", hour),
				TotalPrice:     float64(50 + rng.Intn(200)),
				Status:         status,
			}
			reservation.CreatedAt = now.AddDate(0, 0, -daysAgo)
			reservation.UpdatedAt = reservation.CreatedAt
			DB.Create(&reservation)
		}

		// ── Feed Posts ────────────────────────────
		for p := 0; p < def.PostCount; p++ {
			daysAgo := rng.Intn(30) + 1
			post := models.FeedPost{
				BeautyCenterID: centerID,
				VideoURL:       "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4",
				ThumbnailURL:   fmt.Sprintf("https://picsum.photos/seed/%s-post%d/360/640", slugify(def.Name), p),
				Caption:        fmt.Sprintf("Trabajo #%d en %s ✨", p+1, def.Name),
				Duration:       30 + rng.Intn(60),
				ViewsCount:     100 + rng.Intn(2000),
				LikesCount:     10 + rng.Intn(200),
				IsActive:       true,
			}
			post.CreatedAt = now.AddDate(0, 0, -daysAgo)
			post.UpdatedAt = post.CreatedAt
			DB.Create(&post)
		}

		// ── Favorites (use real client IDs) ──────────
		for f := 0; f < def.FavCount; f++ {
			clientID := demoClientIDs[rng.Intn(len(demoClientIDs))]
			fav := models.Favorite{
				ClientID:       clientID,
				BeautyCenterID: centerID,
			}
			fav.CreatedAt = now.AddDate(0, 0, -(rng.Intn(60) + 1))
			fav.UpdatedAt = fav.CreatedAt
			DB.Create(&fav)
		}

		utils.Log.Infof("[SEED] Created demo center: %s (%d services, %d reviews, %d reservations, %d posts, %d favs)",
			def.Name, svcCount, def.ReviewCount, def.ResCount, def.PostCount, def.FavCount)
	}

	utils.Log.Infof("[SEED] ✅ Seeded %d demo beauty centers in Oruro, Bolivia", len(centers))
}

// slugify converts a name to a simple slug (lowercase, no spaces/special chars).
func slugify(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			result += string(c)
		} else if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else if c == ' ' || c == '&' || c == '\'' {
			result += "-"
		}
	}
	return result
}

// clampFloat clamps a float64 between min and max.
func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
