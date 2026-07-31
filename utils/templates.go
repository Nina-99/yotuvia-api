package utils

import "strings"

type Template struct {
	Name        string
	Description string
	Icon        string
}

// ─── Category Templates ──────────────────────────────────────
// Usados por GET /category/templates y también para localizar
// nombres de categorías existentes.
// ─────────────────────────────────────────────────────────────

var CategoryTemplatesEs = []Template{
	{Name: "Cortes", Description: "Cortes de cabello mujer, hombre e infantil", Icon: "💇"},
	{Name: "Color / Tintura", Description: "Tinte completo, mechas, balayage, reflejos", Icon: "🎨"},
	{Name: "Peinados", Description: "Peinados para eventos, recogidos, ondas", Icon: "✨"},
	{Name: "Tratamientos capilares", Description: "Botox capilar, keratina, hidratación", Icon: "🧴"},
	{Name: "Alisados / Brasileños", Description: "Alisado permanente, progresivo, brasileño", Icon: "🪮"},
	{Name: "Manicura", Description: "Esmaltado semipermanente, capping, tradicional", Icon: "💅"},
	{Name: "Pedicura", Description: "Pedicura completa, semipermanente, spa", Icon: "🦶"},
	{Name: "Uñas esculpidas", Description: "Acrílicas, gel, press-on, diseños", Icon: "💎"},
	{Name: "Depilación corporal", Description: "Depilación con cera, láser", Icon: "🪒"},
	{Name: "Depilación facial", Description: "Cejas, labio superior, rostro completo", Icon: "👁️"},
	{Name: "Masajes", Description: "Masajes relajantes, descontracturantes, reductores", Icon: "💆"},
	{Name: "Limpieza facial", Description: "Limpieza profunda, hidratación, exfoliación", Icon: "🧖"},
	{Name: "Maquillaje", Description: "Maquillaje social, novias, artístico", Icon: "💄"},
	{Name: "Cejas y pestañas", Description: "Laminado, micropigmentación, lifting, extensiones", Icon: "👁️‍🗨️"},
	{Name: "Barba y bigote", Description: "Arreglo de barba, perfilado, afeitado clásico", Icon: "🧔"},
}

var CategoryTemplatesEn = []Template{
	{Name: "Haircuts", Description: "Women's, men's and children's haircuts", Icon: "💇"},
	{Name: "Color / Dye", Description: "Full color, highlights, balayage, roots", Icon: "🎨"},
	{Name: "Hairstyling", Description: "Event hairstyles, updos, waves", Icon: "✨"},
	{Name: "Hair treatments", Description: "Hair botox, keratin, deep conditioning", Icon: "🧴"},
	{Name: "Straightening", Description: "Permanent straightening, Brazilian blowout", Icon: "🪮"},
	{Name: "Manicure", Description: "Gel polish, overlay, classic", Icon: "💅"},
	{Name: "Pedicure", Description: "Full pedicure, gel polish, spa", Icon: "🦶"},
	{Name: "Nail extensions", Description: "Acrylic, gel, press-on, nail art", Icon: "💎"},
	{Name: "Body waxing", Description: "Waxing, laser hair removal", Icon: "🪒"},
	{Name: "Facial waxing", Description: "Eyebrows, upper lip, full face", Icon: "👁️"},
	{Name: "Massages", Description: "Relaxing, deep tissue, slimming", Icon: "💆"},
	{Name: "Facial treatments", Description: "Deep cleansing, hydration, exfoliation", Icon: "🧖"},
	{Name: "Makeup", Description: "Social makeup, bridal, artistic", Icon: "💄"},
	{Name: "Brows & lashes", Description: "Lamination, micropigmentation, lifting, extensions", Icon: "👁️‍🗨️"},
	{Name: "Beard & mustache", Description: "Beard shaping, trimming, classic shave", Icon: "🧔"},
}

var serviceTemplatesEs = map[string][]Template{
	"cortes": {
		{Name: "Corte de cabello mujer", Description: "Corte y peinado para mujer", Icon: "💇‍♀️"},
		{Name: "Corte de cabello hombre", Description: "Corte clásico o moderno para hombre", Icon: "💇‍♂️"},
		{Name: "Corte infantil", Description: "Corte para niños hasta 12 años", Icon: "👶"},
		{Name: "Corte degradado", Description: "Degradado militar, medio o alto", Icon: "✂️"},
		{Name: "Corte con navaja", Description: "Corte texturizado con navaja", Icon: "🪒"},
		{Name: "Corte moderno", Description: "Corte de tendencia actual", Icon: "⭐"},
		{Name: "Rebaje / Capas", Description: "Rebaje y capas para dar volumen", Icon: "📐"},
	},
	"color tintura": {
		{Name: "Tinte completo", Description: "Coloración completa del cabello", Icon: "🎨"},
		{Name: "Tinte de raíces", Description: "Retoque de raíces", Icon: "🔄"},
		{Name: "Mechas", Description: "Mechas finas o gruesas", Icon: "🌟"},
		{Name: "Balayage", Description: "Técnica de mechas a mano alzada", Icon: "✨"},
		{Name: "Reflejos", Description: "Reflejos naturales", Icon: "☀️"},
		{Name: "Decoloración", Description: "Decoloración completa", Icon: "⬜"},
		{Name: "Matización", Description: "Matización de tonos", Icon: "🎯"},
		{Name: "Baño de color", Description: "Baño de color temporal", Icon: "🌈"},
	},
	"peinados": {
		{Name: "Recogido", Description: "Recogido clásico o moderno", Icon: "💁"},
		{Name: "Ondas", Description: "Ondas al agua o marcadas", Icon: "🌊"},
		{Name: "Alisado con plancha", Description: "Alisado con plancha", Icon: "🔧"},
		{Name: "Peinado novia", Description: "Peinado para novias", Icon: "👰"},
		{Name: "Peinado 15 años", Description: "Peinado para fiesta de 15", Icon: "👑"},
		{Name: "Trenzas", Description: "Trenzas clásicas o artísticas", Icon: "🪢"},
		{Name: "Moño", Description: "Moño elegante o casual", Icon: "🎀"},
		{Name: "Semirecogido", Description: "Semirecogido con ondas", Icon: "🌸"},
	},
	"tratamientos capilares": {
		{Name: "Botox capilar", Description: "Tratamiento de keratina", Icon: "💉"},
		{Name: "Keratina", Description: "Alisado con keratina", Icon: "✨"},
		{Name: "Hidratación profunda", Description: "Hidratación intensiva", Icon: "💧"},
		{Name: "Nutrición capilar", Description: "Nutrición con aceites", Icon: "🌿"},
		{Name: "Reconstrucción capilar", Description: "Reconstrucción de fibra capilar", Icon: "🔨"},
		{Name: "Tricología / Caída", Description: "Tratamiento anticaída", Icon: "🌱"},
	},
	"alisados brasilenos": {
		{Name: "Alisado permanente", Description: "Alisado definitivo", Icon: "🔗"},
		{Name: "Alisado progresivo", Description: "Alisado progresivo paso a paso", Icon: "📈"},
		{Name: "Alisado brasileño", Description: "Alisado brasileño tradicional", Icon: "🇧🇷"},
		{Name: "Alisado orgánico", Description: "Alisado sin formol", Icon: "🌿"},
	},
	"manicura": {
		{Name: "Esmaltado semipermanente", Description: "Esmaltado semipermanente", Icon: "💅"},
		{Name: "Capping", Description: "Capping en gel", Icon: "💎"},
		{Name: "Manicura clásica", Description: "Corte y esmaltado tradicional", Icon: "✅"},
		{Name: "Manicura spa", Description: "Manicura con exfoliación y masaje", Icon: "🧖"},
		{Name: "Kapping gel", Description: "Kapping de gel fortalecedor", Icon: "🛡️"},
		{Name: "Esmaltado tradicional", Description: "Esmaltado clásico", Icon: "💅"},
	},
	"pedicura": {
		{Name: "Pedicura completa", Description: "Pedicura completa con esmaltado", Icon: "🦶"},
		{Name: "Pedicura semipermanente", Description: "Pedicura con semipermanente", Icon: "💅"},
		{Name: "Pedicura spa", Description: "Pedicura con exfoliación y masaje", Icon: "🧖"},
		{Name: "Pedicura capping", Description: "Pedicura con capping en gel", Icon: "💎"},
	},
	"unas esculpidas": {
		{Name: "Uñas acrílicas", Description: "Uñas esculpidas en acrílico", Icon: "💎"},
		{Name: "Uñas en gel", Description: "Uñas esculpidas en gel", Icon: "✨"},
		{Name: "Press-on", Description: "Uñas press-on personalizadas", Icon: "📌"},
		{Name: "Esculpidas con tips", Description: "Esculpidas con tips de uña", Icon: "📐"},
		{Name: "Esculpidas sin tips", Description: "Esculpidas con moldes de papel", Icon: "📃"},
		{Name: "Nail art", Description: "Diseños artísticos en uñas", Icon: "🎨"},
	},
	"depilacion corporal": {
		{Name: "Depilación piernas completas", Description: "Depilación completa de piernas", Icon: "🦵"},
		{Name: "Depilación medio pierna", Description: "Depilación de media pierna", Icon: "🦵"},
		{Name: "Depilación axilas", Description: "Depilación de axilas", Icon: "🤚"},
		{Name: "Depilación brazos", Description: "Depilación completa de brazos", Icon: "💪"},
		{Name: "Depilación inglesa", Description: "Depilación de zona íntima", Icon: "🔒"},
		{Name: "Depilación completa", Description: "Depilación de cuerpo completo", Icon: "🧑"},
	},
	"depilacion facial": {
		{Name: "Cejas", Description: "Diseño y depilación de cejas", Icon: "👁️"},
		{Name: "Labio superior", Description: "Depilación de labio superior", Icon: "👄"},
		{Name: "Rostro completo", Description: "Depilación de rostro completo", Icon: "🧑"},
		{Name: "Hilo", Description: "Depilación con hilo", Icon: "🧵"},
		{Name: "Pinza", Description: "Depilación con pinza", Icon: "🔧"},
		{Name: "Cera", Description: "Depilación facial con cera", Icon: "🕯️"},
	},
	"masajes": {
		{Name: "Masaje relajante", Description: "Masaje de relajación corporal", Icon: "💆"},
		{Name: "Masaje descontracturante", Description: "Masaje para contracturas musculares", Icon: "💪"},
		{Name: "Masaje reductor", Description: "Masaje reductores de medidas", Icon: "📏"},
		{Name: "Masaje deportivo", Description: "Masaje pre y post deportivo", Icon: "🏃"},
		{Name: "Drenaje linfático", Description: "Drenaje linfático manual", Icon: "🫧"},
		{Name: "Piedras calientes", Description: "Masaje con piedras volcánicas", Icon: "🪨"},
	},
	"limpieza facial": {
		{Name: "Limpieza facial profunda", Description: "Limpieza facial completa", Icon: "🧼"},
		{Name: "Hidratación facial", Description: "Tratamiento de hidratación facial", Icon: "💧"},
		{Name: "Exfoliación facial", Description: "Exfoliación facial con productos", Icon: "🧴"},
		{Name: "Punta de diamante", Description: "Microdermoabrasión con punta de diamante", Icon: "💎"},
		{Name: "Alta frecuencia", Description: "Tratamiento con alta frecuencia", Icon: "⚡"},
		{Name: "Radiofrecuencia", Description: "Radiofrecuencia facial", Icon: "📡"},
	},
	"maquillaje": {
		{Name: "Maquillaje social", Description: "Maquillaje para eventos sociales", Icon: "💄"},
		{Name: "Maquillaje novia", Description: "Maquillaje para novias", Icon: "👰"},
		{Name: "Maquillaje artístico", Description: "Maquillaje artístico o fantasía", Icon: "🎭"},
		{Name: "Maquillaje 15 años", Description: "Maquillaje para fiesta de 15", Icon: "👑"},
		{Name: "Maquillaje noche", Description: "Maquillaje de noche", Icon: "🌙"},
		{Name: "Maquillaje correctivo", Description: "Maquillaje correctivo profesional", Icon: "🎯"},
	},
	"cejas y pestanas": {
		{Name: "Laminado de cejas", Description: "Laminado y peinado de cejas", Icon: "👁️"},
		{Name: "Micropigmentación de cejas", Description: "Micropigmentación semipermanente", Icon: "✍️"},
		{Name: "Henna de cejas", Description: "Color con henna para cejas", Icon: "🌿"},
		{Name: "Perfilado de cejas", Description: "Diseño y perfilado profesional", Icon: "📐"},
		{Name: "Lifting de pestañas", Description: "Lifting de pestañas natural", Icon: "👁️"},
		{Name: "Extensiones de pestañas", Description: "Extensiones pelo a pelo", Icon: "✨"},
	},
	"barba y bigote": {
		{Name: "Arreglo de barba", Description: "Recorte y perfilado de barba", Icon: "✂️"},
		{Name: "Perfilado", Description: "Perfilado de barba con navaja", Icon: "🪒"},
		{Name: "Afeitado clásico", Description: "Afeitado tradicional con navaja", Icon: "🪒"},
		{Name: "Barba completa", Description: "Servicio completo de barba", Icon: "🧔"},
		{Name: "Recorte de bigote", Description: "Recorte y perfilado de bigote", Icon: "✂️"},
		{Name: "Barbería completa", Description: "Corte + barba + afeitado", Icon: "💈"},
	},
}

var serviceTemplatesEn = map[string][]Template{
	"haircuts": {
		{Name: "Women's haircut", Description: "Women's haircut and styling", Icon: "💇‍♀️"},
		{Name: "Men's haircut", Description: "Classic or modern men's cut", Icon: "💇‍♂️"},
		{Name: "Children's haircut", Description: "Haircut for children up to 12", Icon: "👶"},
		{Name: "Fade haircut", Description: "Military, mid or high fade", Icon: "✂️"},
		{Name: "Textured cut", Description: "Textured cut with razor", Icon: "🪒"},
		{Name: "Modern cut", Description: "Trendy modern haircut", Icon: "⭐"},
		{Name: "Layers", Description: "Layered cut for volume", Icon: "📐"},
	},
	"color dye": {
		{Name: "Full color", Description: "Full hair coloring", Icon: "🎨"},
		{Name: "Root touch-up", Description: "Root color retouch", Icon: "🔄"},
		{Name: "Highlights", Description: "Fine or thick highlights", Icon: "🌟"},
		{Name: "Balayage", Description: "Hand-painted highlights", Icon: "✨"},
		{Name: "Babylights", Description: "Natural babylights", Icon: "☀️"},
		{Name: "Bleach", Description: "Full hair bleach", Icon: "⬜"},
		{Name: "Toning", Description: "Hair toning service", Icon: "🎯"},
		{Name: "Color gloss", Description: "Temporary color gloss", Icon: "🌈"},
	},
	"hairstyling": {
		{Name: "Updo", Description: "Classic or modern updo", Icon: "💁"},
		{Name: "Waves", Description: "Beach waves or defined curls", Icon: "🌊"},
		{Name: "Flat iron", Description: "Hair straightening with flat iron", Icon: "🔧"},
		{Name: "Bridal hairstyle", Description: "Bridal hairstyling", Icon: "👰"},
		{Name: "Quinceañera hair", Description: "Hairstyle for quinceañera", Icon: "👑"},
		{Name: "Braids", Description: "Classic or artistic braids", Icon: "🪢"},
		{Name: "Bun", Description: "Elegant or casual bun", Icon: "🎀"},
	},
	"hair treatments": {
		{Name: "Hair botox", Description: "Keratin smoothing treatment", Icon: "💉"},
		{Name: "Keratin treatment", Description: "Keratin straightening", Icon: "✨"},
		{Name: "Deep conditioning", Description: "Intensive hydration treatment", Icon: "💧"},
		{Name: "Hair nutrition", Description: "Oil-based hair nutrition", Icon: "🌿"},
		{Name: "Hair reconstruction", Description: "Hair fiber reconstruction", Icon: "🔨"},
		{Name: "Hair loss treatment", Description: "Anti-hair loss treatment", Icon: "🌱"},
	},
	"straightening": {
		{Name: "Permanent straightening", Description: "Permanent hair straightening", Icon: "🔗"},
		{Name: "Progressive straightening", Description: "Step-by-step progressive", Icon: "📈"},
		{Name: "Brazilian blowout", Description: "Traditional Brazilian blowout", Icon: "🇧🇷"},
		{Name: "Organic straightening", Description: "Formaldehyde-free straightening", Icon: "🌿"},
	},
	"manicure": {
		{Name: "Gel polish", Description: "Semi-permanent gel polish", Icon: "💅"},
		{Name: "Gel overlay", Description: "Gel overlay natural nails", Icon: "💎"},
		{Name: "Classic manicure", Description: "Classic nail shaping and polish", Icon: "✅"},
		{Name: "Spa manicure", Description: "Manicure with exfoliation and massage", Icon: "🧖"},
		{Name: "Hard gel overlay", Description: "Strengthening hard gel overlay", Icon: "🛡️"},
		{Name: "Regular polish", Description: "Classic nail polish", Icon: "💅"},
	},
	"pedicure": {
		{Name: "Full pedicure", Description: "Complete pedicure with polish", Icon: "🦶"},
		{Name: "Gel pedicure", Description: "Pedicure with gel polish", Icon: "💅"},
		{Name: "Spa pedicure", Description: "Pedicure with exfoliation and massage", Icon: "🧖"},
		{Name: "Gel overlay pedicure", Description: "Pedicure with gel overlay", Icon: "💎"},
	},
	"nail extensions": {
		{Name: "Acrylic nails", Description: "Acrylic nail extensions", Icon: "💎"},
		{Name: "Gel nails", Description: "Gel nail extensions", Icon: "✨"},
		{Name: "Press-on nails", Description: "Custom press-on nails", Icon: "📌"},
		{Name: "Tip extensions", Description: "Extensions with nail tips", Icon: "📐"},
		{Name: "Paper form extensions", Description: "Extensions with paper forms", Icon: "📃"},
		{Name: "Nail art", Description: "Artistic nail designs", Icon: "🎨"},
	},
	"body waxing": {
		{Name: "Full leg wax", Description: "Complete leg waxing", Icon: "🦵"},
		{Name: "Half leg wax", Description: "Half leg waxing", Icon: "🦵"},
		{Name: "Underarm wax", Description: "Underarm waxing", Icon: "🤚"},
		{Name: "Full arm wax", Description: "Complete arm waxing", Icon: "💪"},
		{Name: "Bikini wax", Description: "Bikini area waxing", Icon: "🔒"},
		{Name: "Full body wax", Description: "Full body waxing", Icon: "🧑"},
	},
	"facial waxing": {
		{Name: "Eyebrows", Description: "Eyebrow shaping and waxing", Icon: "👁️"},
		{Name: "Upper lip", Description: "Upper lip waxing", Icon: "👄"},
		{Name: "Full face", Description: "Complete facial waxing", Icon: "🧑"},
		{Name: "Threading", Description: "Facial threading", Icon: "🧵"},
		{Name: "Tweezing", Description: "Precision tweezing", Icon: "🔧"},
		{Name: "Facial wax", Description: "Facial waxing with soft wax", Icon: "🕯️"},
	},
	"massages": {
		{Name: "Relaxation massage", Description: "Full body relaxation massage", Icon: "💆"},
		{Name: "Deep tissue massage", Description: "Deep tissue muscle massage", Icon: "💪"},
		{Name: "Slimming massage", Description: "Body contouring massage", Icon: "📏"},
		{Name: "Sports massage", Description: "Pre and post sports massage", Icon: "🏃"},
		{Name: "Lymphatic drainage", Description: "Manual lymphatic drainage", Icon: "🫧"},
		{Name: "Hot stone massage", Description: "Volcanic hot stone massage", Icon: "🪨"},
	},
	"facial treatments": {
		{Name: "Deep cleansing facial", Description: "Complete deep cleansing facial", Icon: "🧼"},
		{Name: "Hydrating facial", Description: "Hydrating facial treatment", Icon: "💧"},
		{Name: "Facial exfoliation", Description: "Facial exfoliation treatment", Icon: "🧴"},
		{Name: "Diamond tip microdermabrasion", Description: "Diamond tip microdermabrasion", Icon: "💎"},
		{Name: "High frequency", Description: "High frequency facial treatment", Icon: "⚡"},
		{Name: "Radiofrequency", Description: "Radiofrequency facial treatment", Icon: "📡"},
	},
	"makeup": {
		{Name: "Social makeup", Description: "Makeup for social events", Icon: "💄"},
		{Name: "Bridal makeup", Description: "Bridal makeup service", Icon: "👰"},
		{Name: "Artistic makeup", Description: "Artistic or fantasy makeup", Icon: "🎭"},
		{Name: "Quinceañera makeup", Description: "Makeup for quinceañera", Icon: "👑"},
		{Name: "Evening makeup", Description: "Evening makeup", Icon: "🌙"},
		{Name: "Corrective makeup", Description: "Professional corrective makeup", Icon: "🎯"},
	},
	"brows lashes": {
		{Name: "Brow lamination", Description: "Brow lamination and styling", Icon: "👁️"},
		{Name: "Brow micropigmentation", Description: "Semi-permanent eyebrow tattooing", Icon: "✍️"},
		{Name: "Brow henna", Description: "Henna brow tinting", Icon: "🌿"},
		{Name: "Brow shaping", Description: "Professional brow design", Icon: "📐"},
		{Name: "Lash lift", Description: "Natural lash lift", Icon: "👁️"},
		{Name: "Lash extensions", Description: "Individual lash extensions", Icon: "✨"},
	},
	"beard mustache": {
		{Name: "Beard trim", Description: "Beard trimming and shaping", Icon: "✂️"},
		{Name: "Beard shaping", Description: "Beard shaping with straight razor", Icon: "🪒"},
		{Name: "Classic shave", Description: "Traditional straight razor shave", Icon: "🪒"},
		{Name: "Full beard service", Description: "Complete beard service", Icon: "🧔"},
		{Name: "Mustache trim", Description: "Mustache trimming and shaping", Icon: "✂️"},
		{Name: "Full barber service", Description: "Haircut + beard + shave", Icon: "💈"},
	},
}

// ─── Helpers ──────────────────────────────────────────────────

// NormalizeName normalizes a string for key matching (lowercase, no accents, no special chars)
func NormalizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
		"ñ", "n", "ü", "u",
	)
	name = replacer.Replace(name)
	replacer2 := strings.NewReplacer("/", " ", "–", " ", "-", " ", "—", " ")
	name = replacer2.Replace(name)
	words := strings.Fields(name)
	return strings.Join(words, " ")
}

// GetServiceTemplates returns service name suggestions for a given category name
func GetServiceTemplates(categoryName string, lang string) []Template {
	// Localize the category name to the target language first.
	// This handles DB-stored names in Spanish when English is requested.
	searchName := categoryName
	if lang == "en" {
		if translated, _ := LocalizeCategoryName(categoryName, "en"); translated != categoryName {
			searchName = translated
		}
	} else if lang == "es" {
		if translated, _ := LocalizeCategoryName(categoryName, "es"); translated != categoryName {
			searchName = translated
		}
	}

	normalized := NormalizeName(searchName)

	// Try language-specific map first
	var primaryMap, fallbackMap map[string][]Template
	if lang == "en" {
		primaryMap = serviceTemplatesEn
		fallbackMap = serviceTemplatesEs
	} else {
		primaryMap = serviceTemplatesEs
		fallbackMap = serviceTemplatesEn
	}

	// Exact match
	if suggestions, ok := primaryMap[normalized]; ok {
		return suggestions
	}

	// Partial match in primary
	for key, suggestions := range primaryMap {
		if strings.Contains(normalized, key) || strings.Contains(key, normalized) {
			return suggestions
		}
	}

	// Fallback search
	if fallbackMap != nil {
		if suggestions, ok := fallbackMap[normalized]; ok {
			return suggestions
		}
		for key, suggestions := range fallbackMap {
			if strings.Contains(normalized, key) || strings.Contains(key, normalized) {
				return suggestions
			}
		}
	}

	return nil
}

// LocalizeCategoryName translates a known category name to the requested language.
// Returns the original name if no translation is found.
func LocalizeCategoryName(name string, lang string) (string, string) {
	if lang == "en" {
		// Current name is in es → find en match
		for i, t := range CategoryTemplatesEs {
			if NormalizeName(t.Name) == NormalizeName(name) {
				return CategoryTemplatesEn[i].Name, CategoryTemplatesEn[i].Description
			}
		}
		// Try exact match on names
		for _, t := range CategoryTemplatesEn {
			if NormalizeName(t.Name) == NormalizeName(name) {
				return t.Name, t.Description
			}
		}
	} else {
		// Current name is in en → find es match
		for i, t := range CategoryTemplatesEn {
			if NormalizeName(t.Name) == NormalizeName(name) {
				return CategoryTemplatesEs[i].Name, CategoryTemplatesEs[i].Description
			}
		}
		for _, t := range CategoryTemplatesEs {
			if NormalizeName(t.Name) == NormalizeName(name) {
				return t.Name, t.Description
			}
		}
	}
	return name, ""
}

// CategoryIcon returns the icon for a category name, or empty string if not found.
func CategoryIcon(name string) string {
	for _, t := range CategoryTemplatesEs {
		if NormalizeName(t.Name) == NormalizeName(name) {
			return t.Icon
		}
	}
	for _, t := range CategoryTemplatesEn {
		if NormalizeName(t.Name) == NormalizeName(name) {
			return t.Icon
		}
	}
	return ""
}

func CategoryTemplates(lang string) []Template {
	if lang == "en" {
		return CategoryTemplatesEn
	}
	return CategoryTemplatesEs
}
