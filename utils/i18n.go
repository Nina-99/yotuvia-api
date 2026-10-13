package utils

import "strings"

// LocalizedValue resolves a translatable field for the requested language.
//
// The base column is the canonical/fallback value (what the owner typed first);
// the i18n map holds per-locale overrides. A missing/blank entry for `lang`
// falls back to the base, so a viewer never sees an empty field just because a
// translation was not provided.
func LocalizedValue(base string, i18n map[string]string, lang string) string {
	if lang != "" && i18n != nil {
		if v, ok := i18n[lang]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return base
}

// NormalizeI18n drops blank entries and returns nil for an empty map so the
// stored payload stays tight (no `{"en":""}` noise).
func NormalizeI18n(i18n map[string]string) map[string]string {
	if len(i18n) == 0 {
		return nil
	}
	out := make(map[string]string, len(i18n))
	for k, v := range i18n {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
