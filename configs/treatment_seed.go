package configs

import (
	"sort"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
)

// treatmentCategoryDef is one entry of the BASE global treatment vocabulary:
// a category and the treatment names it groups. It is data, not behavior, so
// extending the vocabulary is a one-line change.
type treatmentCategoryDef struct {
	Name        string
	Description string
	Icon        string
	Treatments  []string
}

// treatmentTaxonomy is the base global vocabulary of common beauty/barbershop
// treatments. It is intentionally center-agnostic: the same "Corte clásico"
// exists for every center. Names are the canonical Spanish terms plus the
// legacy service names already present in the database, so the best-effort
// backfill can link real rows instead of guessing.
var treatmentTaxonomy = []treatmentCategoryDef{
	{
		Name:        "Peluquería",
		Description: "Cortes, color, peinados y tratamientos capilares.",
		Icon:        "💇",
		Treatments: []string{
			"Corte clásico",
			"Corte + barba",
			"Corte de cabello",
			"Corte de cabello (dama)",
			"Corte infantil",
			"Corte degradado",
			"Peinado",
			"Brushing",
			"Tinte completo",
			"Full color",
			"Balayage",
			"Mechas",
			"Alisado",
			"Ondas",
			"Tratamiento capilar",
			"Hidratación capilar",
			"Women's haircut",
		},
	},
	{
		Name:        "Manicuría",
		Description: "Manicura, pedicura y uñas esculpidas.",
		Icon:        "💅",
		Treatments: []string{
			"Manicuría semipermanente",
			"Uñas acrílicas",
			"Nail art",
			"Manicura clásica",
			"Pedicura",
			"Kapping",
			"Retiro de esmalte",
		},
	},
	{
		Name:        "Maquillaje",
		Description: "Maquillaje social, de novia y artístico.",
		Icon:        "💄",
		Treatments: []string{
			"Maquillaje social",
			"Maquillaje de novia",
			"Maquillaje artístico",
			"Maquillaje de día",
			"Pestañas postizas",
		},
	},
	{
		Name:        "Estética facial",
		Description: "Tratamientos faciales, depilación y masajes.",
		Icon:        "✨",
		Treatments: []string{
			"Limpieza facial",
			"Exfoliación facial",
			"Hidratación facial",
			"Depilación de piernas",
			"Depilación con cera",
			"Depilación de cejas",
			"Depilación facial",
			"Masaje relajante",
			"Masaje descontracturante",
		},
	},
	{
		Name:        "Barbería",
		Description: "Cortes masculinos y cuidado de barba.",
		Icon:        "💈",
		Treatments: []string{
			"Afeitado a navaja",
			"Arreglo de barba",
			"Perfilado de barba",
			"Ritual de barba",
		},
	},
}

// normalizeTreatmentName is the matching normalizer for the taxonomy: lowercase,
// trimmed, accents stripped and internal whitespace collapsed. "  CORTE CLÁSICO "
// and "corte clasico" both normalize to "corte clasico", so the backfill pairs
// real service names with vocabulary terms regardless of casing or accents.
func normalizeTreatmentName(name string) string {
	return utils.NormalizeName(name)
}

// SeedTreatmentTaxonomy creates the BASE global treatment catalog. It follows
// the same create-or-heal pattern as SeedDefaultMissions / SeedMarketConfigs:
// a second boot finds every row and creates nothing, so it is idempotent.
// Drifted rows (description/icon/active flag) are healed back to canonical.
func SeedTreatmentTaxonomy() {
	createdCategories := 0
	createdTreatments := 0

	for _, def := range treatmentTaxonomy {
		var category models.TreatmentCategory
		found := DB.Where("name = ?", def.Name).First(&category).Error == nil
		if !found {
			category = models.TreatmentCategory{
				Name:        def.Name,
				Description: def.Description,
				Icon:        def.Icon,
			}
			if err := DB.Create(&category).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to create treatment category %q: %v", def.Name, err)
				continue
			}
			createdCategories++
		} else if category.Description != def.Description || category.Icon != def.Icon {
			if err := DB.Model(&category).Updates(map[string]interface{}{
				"description": def.Description,
				"icon":        def.Icon,
			}).Error; err != nil {
				utils.Log.Errorf("[SEED] Failed to heal treatment category %q: %v", def.Name, err)
			}
		}

		for _, name := range def.Treatments {
			var existing models.Treatment
			found := DB.Where("treatment_category_id = ? AND name = ?", category.ID, name).First(&existing).Error == nil
			if !found {
				treatment := models.Treatment{
					TreatmentCategoryID: category.ID,
					Name:                name,
					IsActive:            true,
				}
				if err := DB.Create(&treatment).Error; err != nil {
					utils.Log.Errorf("[SEED] Failed to create treatment %q: %v", name, err)
					continue
				}
				createdTreatments++
			} else if !existing.IsActive {
				if err := DB.Model(&existing).Update("is_active", true).Error; err != nil {
					utils.Log.Errorf("[SEED] Failed to reactivate treatment %q: %v", name, err)
				}
			}
		}
	}

	utils.Log.Infof("[SEED] Treatment taxonomy ready (created %d categories, %d treatments this run)",
		createdCategories, createdTreatments)
}

// TreatmentBackfillStats reports the outcome of the best-effort service→treatment
// backfill. MatchRate is linked/eligible as a percentage; UnmatchedNames are the
// distinct service names the vocabulary could not explain, sorted for stable
// reporting.
type TreatmentBackfillStats struct {
	TotalServices  int
	Eligible       int
	Linked         int
	AlreadyLinked  int
	MatchRate      float64
	UnmatchedNames []string
}

// BackfillServiceTreatments links existing services to the global vocabulary by
// NORMALIZED name. It is deliberately conservative:
//
//   - only rows with a NULL TreatmentID are considered, so an existing link is
//     NEVER overwritten;
//   - a row whose name matches nothing is logged and skipped, never aborting
//     startup;
//   - linking is batched (one UPDATE per matched treatment), so a large catalog
//     does not fan out one write per service.
//
// It returns the match rate so the quality of the vocabulary seed is measurable.
func BackfillServiceTreatments() TreatmentBackfillStats {
	var stats TreatmentBackfillStats

	var treatments []models.Treatment
	if err := DB.Find(&treatments).Error; err != nil {
		utils.Log.Errorf("[SEED] Treatment backfill: could not load treatments: %v", err)
		return stats
	}
	byName := make(map[string]uint, len(treatments))
	for _, t := range treatments {
		byName[normalizeTreatmentName(t.Name)] = t.ID
	}

	var total int64
	if err := DB.Model(&models.Service{}).Count(&total).Error; err != nil {
		utils.Log.Errorf("[SEED] Treatment backfill: could not count services: %v", err)
		return stats
	}
	stats.TotalServices = int(total)

	var services []models.Service
	if err := DB.Where("treatment_id IS NULL").Find(&services).Error; err != nil {
		utils.Log.Errorf("[SEED] Treatment backfill: could not load unlinked services: %v", err)
		return stats
	}
	stats.Eligible = len(services)
	stats.AlreadyLinked = stats.TotalServices - stats.Eligible

	pending := make(map[uint][]uint) // treatmentID → service IDs to link
	unmatched := make(map[string]struct{})
	for _, svc := range services {
		treatmentID, ok := byName[normalizeTreatmentName(svc.Name)]
		if !ok {
			unmatched[svc.Name] = struct{}{}
			continue
		}
		pending[treatmentID] = append(pending[treatmentID], svc.ID)
	}

	for treatmentID, ids := range pending {
		res := DB.Model(&models.Service{}).
			Where("id IN ? AND treatment_id IS NULL", ids).
			Update("treatment_id", treatmentID)
		if res.Error != nil {
			utils.Log.Errorf("[SEED] Treatment backfill: failed to link %d services to treatment %d: %v",
				len(ids), treatmentID, res.Error)
			continue
		}
		stats.Linked += int(res.RowsAffected)
	}

	if stats.Eligible > 0 {
		stats.MatchRate = float64(stats.Linked) / float64(stats.Eligible) * 100
	}
	for name := range unmatched {
		stats.UnmatchedNames = append(stats.UnmatchedNames, name)
	}
	sort.Strings(stats.UnmatchedNames)

	utils.Log.Infof("[SEED] Treatment backfill: linked %d/%d previously unlinked services (%.1f%%), %d already linked, %d unmatched distinct names",
		stats.Linked, stats.Eligible, stats.MatchRate, stats.AlreadyLinked, len(stats.UnmatchedNames))
	if len(stats.UnmatchedNames) > 0 {
		utils.Log.Infof("[SEED] Treatment backfill unmatched service names: %v", stats.UnmatchedNames)
	}
	return stats
}
