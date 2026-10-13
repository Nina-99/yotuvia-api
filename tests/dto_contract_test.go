package tests

// Wire-contract guard: proves the JSON the Go API emits still matches the
// TypeScript DTOs the mobile app declares.
//
// Why this exists: the mobile wire types live in a SEPARATE git repo and drift
// silently. Go encoding/json happily emits a key the client interface never
// declares, and TypeScript happily reads a field the interface never declared
// (the value is `any` at the axios boundary). Six fields were lost this way in
// one session — likes_count, notification personnel_id/beauty_center_id, a
// personnel `name` that never existed in Go, treatment_id (both directions),
// book_taps and the reservation line's personnel_id — each found late and by
// accident. This test turns that class of bug into a red build.
//
// Scope: a curated, documented table of Go response structs that have a TS
// counterpart. It is a DEV-TIME guard, not a CI guard: the sibling repo lives
// at ../yotuvia-mobile (override with YOTUVIA_MOBILE_DIR). When that repo is absent
// the test SKIPS with a clear message instead of failing.
//
// Direction: BOTH. A Go key missing from its TS interface fails ("Go emits a
// field the client cannot see"), and a TS key with no Go counterpart fails
// ("the client reads a field the API never sends"). Every current difference is
// recorded in the per-binding allow-list with a one-line reason, so the guard
// stays strict against NEW drift while the existing backlog stays visible.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
)

// ─────────────────────────────────────────────────────────────
//  COVERAGE TABLE
// ─────────────────────────────────────────────────────────────

// dtoBinding ties one Go response struct to the TypeScript interface that is
// supposed to mirror its wire shape.
type dtoBinding struct {
	// Label is the human name used in failure output.
	Label string
	// GoType is the reflected response (or model) struct.
	GoType reflect.Type
	// TSFile is relative to the mobile repo root.
	TSFile string
	// TSInterface is the interface that must declare the same keys.
	TSInterface string
	// TSNestedField, when set, narrows the comparison to the object literal of
	// that member (arrays are unwrapped). Used for embedded/sub-shapes.
	TSNestedField string
	// GoOnly maps a Go JSON key that is deliberately absent from the TS
	// interface to a one-line reason. Reasons starting with "KNOWN GAP" mark
	// real drift found by this guard; "deliberate" marks intended omissions.
	GoOnly map[string]string
	// TSOnly maps a TS key with no Go counterpart to a one-line reason.
	TSOnly map[string]string
}

func dtoBindings() []dtoBinding {
	return []dtoBinding{
		{
			Label:       "FeedPostResponse -> FeedPostItem",
			GoType:      reflect.TypeOf(response.FeedPostResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "FeedPostItem",
		},
		{
			Label:       "ReservationResponse -> ReservationResponse (client DTO)",
			GoType:      reflect.TypeOf(response.ReservationResponse{}),
			TSFile:      "src/data/dtos/Reservation.dto.ts",
			TSInterface: "ReservationResponse",
			GoOnly: map[string]string{
				"client":         "KNOWN GAP: Go always serialises the ClientLite object for the detail modal; the client-side TS interface omits it, so the modal reads `client` from an untyped value.",
				"personnel":      "KNOWN GAP: Go always serialises the PersonnelLite object; the client-side TS interface omits it. AdminReservationResponse declares it, the client mirror does not.",
				"personnel_id":   "KNOWN GAP: Go emits the derived lead professional id; the client TS interface omits it, so client code cannot see the lead id it gets from the API (AdminReservationResponse declares it).",
				"source_post_id": "KNOWN GAP: Go emits the credited feed post id (omitempty); no client TS interface declares it, so the video->booking attribution is invisible to the app.",
			},
			TSOnly: map[string]string{
				"beauty_center": "KNOWN GAP: the client TS interface declares a nested beauty_center object the Go ReservationResponse never emits; the mapper falls back to beauty_center_name but the declared shape is fictional.",
				"created_at":    "KNOWN GAP: the client TS interface declares created_at; the Go ReservationResponse has no CreatedAt field, so the value is always undefined.",
			},
		},
		{
			Label:         "ReservationServiceResponse -> ReservationResponse.services[]",
			GoType:        reflect.TypeOf(response.ReservationServiceResponse{}),
			TSFile:        "src/data/dtos/Reservation.dto.ts",
			TSInterface:   "ReservationResponse",
			TSNestedField: "services",
		},
		{
			Label:       "NotificationResponse -> NotificationResponse",
			GoType:      reflect.TypeOf(response.NotificationResponse{}),
			TSFile:      "src/data/dtos/Notification.dto.ts",
			TSInterface: "NotificationResponse",
		},
		{
			Label:       "PersonnelResponse -> PersonnelProfileItem",
			GoType:      reflect.TypeOf(response.PersonnelResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "PersonnelProfileItem",
		},
		{
			Label:       "ReviewResponse -> ReviewResponse",
			GoType:      reflect.TypeOf(response.ReviewResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "ReviewResponse",
			GoOnly: map[string]string{
				"beauty_center": "KNOWN GAP: Go always serialises the full BeautyCenterResponse; the client TS interface omits it, so review rows cannot read the centre they belong to without a second fetch.",
			},
		},
		{
			Label:       "ServiceResponse -> ServiceItem",
			GoType:      reflect.TypeOf(response.ServiceResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "ServiceItem",
			GoOnly: map[string]string{
				"treatment_id": "KNOWN GAP: Go emits the nullable global-treatment link (omitempty); ServiceItem does not declare it, so the service<->treatment link is invisible on the client (the domain Service entity does declare treatmentId).",
				"category":     "KNOWN GAP: Go embeds the CategoryResponse (omitempty); ServiceItem declares no nested category, so the server-provided category object is dropped.",
			},
		},
		{
			Label:       "models.WaitlistEntry -> WaitlistEntryResponse",
			GoType:      reflect.TypeOf(models.WaitlistEntry{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "WaitlistEntryResponse",
			GoOnly: map[string]string{
				"updated_at": "deliberate: GORM bookkeeping, not part of the client contract.",
				"client":     "deliberate: the client already knows its own identity; the listing does not declare the nested Client.",
				"deleted_at": "deliberate: soft-delete marker (omitempty), never meaningful to the client.",
			},
		},
		{
			Label:       "AvailabilitySlotResult -> AvailabilitySearchItemResponse",
			GoType:      reflect.TypeOf(response.AvailabilitySlotResult{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "AvailabilitySearchItemResponse",
		},
		{
			Label:       "AvailabilitySearchResponse -> AvailabilitySearchResponse",
			GoType:      reflect.TypeOf(response.AvailabilitySearchResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "AvailabilitySearchResponse",
		},
		{
			Label:       "TreatmentSummary -> TreatmentCatalogItemResp",
			GoType:      reflect.TypeOf(response.TreatmentSummary{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "TreatmentCatalogItemResp",
		},
		{
			Label:       "TreatmentCategoryResponse -> TreatmentCatalogCategoryResp",
			GoType:      reflect.TypeOf(response.TreatmentCategoryResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "TreatmentCatalogCategoryResp",
		},
		{
			Label:       "TreatmentCatalogResponse -> TreatmentCatalogResponse",
			GoType:      reflect.TypeOf(response.TreatmentCatalogResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "TreatmentCatalogResponse",
		},
		{
			Label:       "ClientResponse -> ClientProfile",
			GoType:      reflect.TypeOf(response.ClientResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "ClientProfile",
		},
		{
			Label:       "BeautyCenterResponse -> BeautyCenter",
			GoType:      reflect.TypeOf(response.BeautyCenterResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "BeautyCenter",
			GoOnly: map[string]string{
				"url":                     "KNOWN GAP: Go emits the public url; BeautyCenter does not declare it.",
				"deposit_required":        "KNOWN GAP: Go emits the deposit policy flags the booking flow needs; BeautyCenter declares none of the deposit_* keys.",
				"deposit_amount":          "KNOWN GAP: deposit policy not declared on the client BeautyCenter.",
				"deposit_percent":         "KNOWN GAP: deposit policy not declared on the client BeautyCenter.",
				"deposit_trust_threshold": "KNOWN GAP: deposit policy not declared on the client BeautyCenter.",
				"deposit_hold_minutes":    "KNOWN GAP: deposit policy not declared on the client BeautyCenter.",
				"deposit_qr_url":          "KNOWN GAP: deposit QR not declared on the client BeautyCenter.",
			},
		},
		{
			Label:       "BeautyCenterImageResp -> BeautyCenterImageResp",
			GoType:      reflect.TypeOf(response.BeautyCenterImageResp{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "BeautyCenterImageResp",
		},
		{
			Label:       "WorkingHourResp -> WorkingHourResp",
			GoType:      reflect.TypeOf(response.WorkingHourResp{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "WorkingHourResp",
		},
		{
			Label:       "AvailableSlotsResponse -> AvailableSlotsResponse",
			GoType:      reflect.TypeOf(response.AvailableSlotsResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "AvailableSlotsResponse",
		},
		{
			Label:       "AvailableSlot -> AvailableSlot",
			GoType:      reflect.TypeOf(response.AvailableSlot{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "AvailableSlot",
		},
		{
			Label:       "RankingResponse -> RankingItem",
			GoType:      reflect.TypeOf(response.RankingResponse{}),
			TSFile:      "src/data/sources/api.ts",
			TSInterface: "RankingItem",
		},
	}
}

// ─────────────────────────────────────────────────────────────
//  GO SIDE — reflection over the real structs
// ─────────────────────────────────────────────────────────────

// goJSONKeys returns every JSON key the struct type can emit, including the
// fields promoted by anonymous/embedded structs. It honours `json:"-"` and
// treats `omitempty` as "may still be emitted" (the guard is about possibility,
// not about a particular value).
func goJSONKeys(t reflect.Type) []string {
	set := map[string]bool{}
	collectGoJSONKeys(t, set)
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectGoJSONKeys(t reflect.Type, out map[string]bool) {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		switch {
		case name == "-":
			continue
		case name != "":
			out[name] = true
		case f.Anonymous:
			// encoding/json flattens an untagged embedded struct into the
			// parent, so its keys are the parent's keys.
			collectGoJSONKeys(f.Type, out)
		default:
			out[f.Name] = true
		}
	}
}

// ─────────────────────────────────────────────────────────────
//  TS SIDE — a small, honest interface parser
// ─────────────────────────────────────────────────────────────

// tsParseResult carries the keys found plus every construct the parser did not
// understand, so a caller can report the blind spots instead of hiding them.
type tsParseResult struct {
	Keys     []string
	Warnings []string
}

// parseTSInterface extracts the top-level member names of a named TypeScript
// interface. It understands multiline members, optional `?`, unions, arrays,
// generics, nested object literals, `readonly`, quoted keys and both comment
// styles. Method signatures and index signatures are skipped and reported.
func parseTSInterface(src, name string) (tsParseResult, error) {
	body, err := tsInterfaceBody(src, name)
	if err != nil {
		return tsParseResult{}, err
	}
	return tsKeysFromMembers(body), nil
}

// parseTSNestedObjectKeys narrows to one member of an interface and returns the
// keys of its object literal (arrays are unwrapped). It is how the reservation
// service line is compared.
func parseTSNestedObjectKeys(src, name, field string) (tsParseResult, error) {
	body, err := tsInterfaceBody(src, name)
	if err != nil {
		return tsParseResult{}, err
	}
	for _, member := range topLevelTSMembers(body) {
		key, isMethod := tsMemberKey(member)
		if isMethod || key != field {
			continue
		}
		typeExpr := tsMemberType(member)
		return tsObjectKeys(typeExpr), nil
	}
	return tsParseResult{}, fmt.Errorf("field %q not found in interface %s", field, name)
}

func tsInterfaceBody(src, name string) (string, error) {
	needle := regexp.MustCompile(`interface\s+` + regexp.QuoteMeta(name) + `\b`)
	loc := needle.FindStringIndex(src)
	if loc == nil {
		return "", fmt.Errorf("interface %s not found", name)
	}
	brace := strings.IndexByte(src[loc[1]:], '{')
	if brace < 0 {
		return "", fmt.Errorf("interface %s has no body", name)
	}
	start := loc[1] + brace
	depth := 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start+1 : i], nil
			}
		}
	}
	return "", fmt.Errorf("unbalanced braces for interface %s", name)
}

// topLevelTSMembers splits an object-literal body into its depth-0 members.
func topLevelTSMembers(body string) []string {
	var members []string
	var cur strings.Builder
	depth := 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch c {
		case '{', '[', '(', '<':
			depth++
		case '}', ']', ')', '>':
			if depth > 0 {
				depth--
			}
		case '"', '\'', '`':
			cur.WriteByte(c)
			quote := c
			i++
			for i < len(body) {
				cur.WriteByte(body[i])
				if body[i] == '\\' && quote != '`' {
					i++
					if i < len(body) {
						cur.WriteByte(body[i])
					}
				} else if body[i] == quote {
					break
				}
				i++
			}
			continue
		}
		if depth == 0 && c == '\n' {
			trimmed := strings.TrimSpace(cur.String())
			if trimmed == "" {
				continue
			}
			// Do not split a member whose type continues on the next line
			// (multiline unions / intersections / generics).
			if strings.HasSuffix(trimmed, "|") || strings.HasSuffix(trimmed, "&") ||
				strings.HasSuffix(trimmed, ":") || strings.HasSuffix(trimmed, ",") ||
				strings.HasSuffix(trimmed, "<") || strings.HasSuffix(trimmed, "(") ||
				strings.HasSuffix(trimmed, "[") ||
				nextNonSpace(body, i+1) == '|' || nextNonSpace(body, i+1) == '&' {
				cur.WriteByte(' ')
				continue
			}
			members = append(members, trimmed)
			cur.Reset()
			continue
		}
		if depth == 0 && (c == ';' || c == ',') {
			members = append(members, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if strings.TrimSpace(cur.String()) != "" {
		members = append(members, cur.String())
	}
	return members
}

// tsMemberKey extracts the declared name of a member. Returns isMethod=true for
// signatures the guard intentionally ignores.
func tsMemberKey(member string) (key string, isMethod bool) {
	m := strings.TrimSpace(member)
	m = strings.TrimPrefix(m, "readonly ")
	m = strings.TrimPrefix(m, "public ")
	m = strings.TrimPrefix(m, "private ")
	if m == "" {
		return "", true
	}
	if strings.HasPrefix(m, "[") { // index signature
		return "", true
	}
	cut := len(m)
	for i := 0; i < len(m); i++ {
		if m[i] == '?' || m[i] == ':' || m[i] == '(' {
			cut = i
			break
		}
	}
	if cut == len(m) {
		return "", true
	}
	if m[cut] == '(' { // method signature
		return "", true
	}
	key = strings.TrimSpace(m[:cut])
	key = strings.Trim(key, `"'`)
	if key == "" {
		return "", true
	}
	return key, false
}

// tsMemberType returns the type expression after the member's colon.
func tsMemberType(member string) string {
	m := strings.TrimSpace(member)
	idx := strings.IndexByte(m, ':')
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(m[idx+1:])
}

// tsObjectKeys parses a type expression and returns the keys of its FIRST
// top-level object literal (arrays/generics are unwrapped). Non-object types
// yield no keys.
func tsObjectKeys(typeExpr string) tsParseResult {
	open := strings.IndexByte(typeExpr, '{')
	if open < 0 {
		return tsParseResult{}
	}
	depth := 0
	body := typeExpr
	for i := open; i < len(typeExpr); i++ {
		switch typeExpr[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				body = typeExpr[open+1 : i]
				i = len(typeExpr)
			}
		}
	}
	return tsKeysFromMembers(body)
}

// tsKeysFromMembers turns an object-literal body into the set of declared field
// names, reporting the constructs it could not classify.
func tsKeysFromMembers(body string) tsParseResult {
	res := tsParseResult{}
	for _, member := range topLevelTSMembers(body) {
		key, isMethod := tsMemberKey(member)
		if isMethod {
			if strings.TrimSpace(member) != "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("skipped non-field member %q", strings.TrimSpace(member)))
			}
			continue
		}
		res.Keys = append(res.Keys, key)
	}
	sort.Strings(res.Keys)
	return res
}

// ─────────────────────────────────────────────────────────────
//  MOBILE REPO RESOLUTION
// ─────────────────────────────────────────────────────────────

// mobileRepoDir resolves the sibling mobile checkout. YOTUVIA_MOBILE_DIR wins;
// otherwise it defaults to ../../yotuvia-mobile relative to this test file, which
// is where the two repos sit side by side.
func mobileRepoDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("YOTUVIA_MOBILE_DIR"); dir != "" {
		return dir
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path to locate the mobile repo")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "yotuvia-mobile"))
}

// ─────────────────────────────────────────────────────────────
//  TEST
// ─────────────────────────────────────────────────────────────

// TestWireDTOContract is the guard. It is skipped, not failed, when the sibling
// mobile checkout is absent: it protects local/branch work against silent DTO
// drift and is not meant to gate a CI image that only carries the Go repo.
func TestWireDTOContract(t *testing.T) {
	root := mobileRepoDir(t)
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		t.Skipf("SKIP: mobile repo not found at %s (set YOTUVIA_MOBILE_DIR to override). "+
			"This dev-time wire guard needs ../yotuvia-mobile to compare against the TS DTOs.", root)
	}

	cache := map[string]string{}
	readTS := func(rel string) (string, error) {
		if src, ok := cache[rel]; ok {
			return src, nil
		}
		abs := filepath.Join(root, rel)
		b, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", abs, err)
		}
		src := stripTSComments(string(b))
		cache[rel] = src
		return src, nil
	}

	for _, b := range dtoBindings() {
		b := b
		t.Run(b.Label, func(t *testing.T) {
			src, err := readTS(b.TSFile)
			if err != nil {
				t.Fatalf("cannot read TS DTO file: %v", err)
			}

			var tsRes tsParseResult
			if b.TSNestedField != "" {
				tsRes, err = parseTSNestedObjectKeys(src, b.TSInterface, b.TSNestedField)
			} else {
				tsRes, err = parseTSInterface(src, b.TSInterface)
			}
			if err != nil {
				t.Fatalf("cannot parse TS interface: %v", err)
			}
			for _, w := range tsRes.Warnings {
				t.Logf("TS parser did not understand: %s (%s :: %s)", w, b.TSFile, b.TSInterface)
			}

			goKeys := goJSONKeys(b.GoType)
			goSet := toStringSet(goKeys)
			tsSet := toStringSet(tsRes.Keys)

			// Go -> TS: the API emits a key the client DTO cannot see.
			var goOnly []string
			for _, k := range goKeys {
				if tsSet[k] {
					continue
				}
				if _, allowed := b.GoOnly[k]; allowed {
					continue
				}
				goOnly = append(goOnly, k)
			}

			// TS -> Go: the client declares a key the API never sends.
			var tsOnly []string
			for _, k := range tsRes.Keys {
				if goSet[k] {
					continue
				}
				if _, allowed := b.TSOnly[k]; allowed {
					continue
				}
				tsOnly = append(tsOnly, k)
			}

			if len(goOnly) > 0 {
				sort.Strings(goOnly)
				t.Errorf("WIRE DTO DRIFT (Go emits, TS does not declare)\n"+
					"  Go struct       : %s\n"+
					"  TS interface    : %s\n"+
					"  TS file         : %s\n"+
					"  missing in TS   : %s\n"+
					"  fix             : declare the key(s) in the TS interface, or add a reasoned allow-list entry",
					b.GoType.Name(), b.TSInterface, b.TSFile, strings.Join(goOnly, ", "))
			}
			if len(tsOnly) > 0 {
				sort.Strings(tsOnly)
				t.Errorf("WIRE DTO DRIFT (TS declares, Go never emits)\n"+
					"  Go struct       : %s\n"+
					"  TS interface    : %s\n"+
					"  TS file         : %s\n"+
					"  phantom in TS   : %s\n"+
					"  fix             : remove the key(s) from the TS interface, or add a reasoned allow-list entry",
					b.GoType.Name(), b.TSInterface, b.TSFile, strings.Join(tsOnly, ", "))
			}
		})
	}
}

// nextNonSpace returns the first non-space byte at or after i, or 0.
func nextNonSpace(s string, i int) byte {
	for ; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return s[i]
		}
	}
	return 0
}

// TestTSParserRobustness pins the parser's behaviour on the TypeScript
// constructs the real DTOs use, plus the ones it deliberately reports as
// unsupported, so a future "improvement" cannot silently change what is
// considered declared.
func TestTSParserRobustness(t *testing.T) {
	const sample = `
/** block comment with interface Noise { fake: 1 } */
export interface Sample<T> {
  // line comment
  id: number;              // trailing comment
  name?: string;
  status: "a" | "b" | null;
  nullable: number | null;
  tags: string[];
  nested: {
    a: number;
    b?: { c: string };
  };
  list: Array<{ x: number; y: number }>;
  multi:
    | "one"
    | "two";
  readonly fixed: boolean;
  [key: string]: unknown;
  doThing(): void;
}
`
	res, err := parseTSInterface(stripTSComments(sample), "Sample")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	want := []string{"fixed", "id", "list", "multi", "name", "nested", "nullable", "status", "tags"}
	if got := strings.Join(res.Keys, ","); got != strings.Join(want, ",") {
		t.Errorf("keys mismatch\n got: %v\nwant: %v", res.Keys, want)
	}
	if len(res.Warnings) != 2 {
		t.Errorf("expected 2 parser warnings (index signature + method), got %d: %v", len(res.Warnings), res.Warnings)
	}

	// The comment inside the block comment must not leak into the body.
	if _, err := parseTSInterface(stripTSComments(sample), "Noise"); err == nil {
		t.Error("parser found an interface inside a comment; comment stripping is broken")
	}

	// Nested narrowing unwraps arrays and object literals.
	nested, err := parseTSNestedObjectKeys(stripTSComments(sample), "Sample", "list")
	if err != nil {
		t.Fatalf("nested parse failed: %v", err)
	}
	if got := strings.Join(nested.Keys, ","); got != "x,y" {
		t.Errorf("nested keys mismatch: got %v want [x y]", nested.Keys)
	}
}

func toStringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		set[it] = true
	}
	return set
}

// ─────────────────────────────────────────────────────────────
//  COMMENT STRIPPER (string-literal aware)
// ─────────────────────────────────────────────────────────────

// stripTSComments removes // and /* */ comments while preserving string and
// template literals, so URLs like `https://x` inside a literal survive.
func stripTSComments(src string) string {
	var b strings.Builder
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		if c == '/' && i+1 < n && src[i+1] == '/' {
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < n && src[i+1] == '*' {
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote := c
			b.WriteByte(c)
			i++
			for i < n {
				b.WriteByte(src[i])
				if src[i] == '\\' && quote != '`' {
					i++
					if i < n {
						b.WriteByte(src[i])
					}
				} else if src[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}
