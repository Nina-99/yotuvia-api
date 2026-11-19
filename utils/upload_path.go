package utils

import "regexp"

// storedReceiptPathRe matches the EXACT shape this API emits for an uploaded
// receipt or payment QR: an opaque UUID filename under /uploads/receipts/ with
// an extension the uploader can actually produce.
//
// Anything a client sends back must match this. Accepting a bare URL is unsafe:
// it lets a client point the centre's payment review screen at an
// attacker-controlled host (IP/User-Agent beacon, phishing redirect) or at a
// scheme like `javascript:`/`data:`.
var storedReceiptPathRe = regexp.MustCompile(
	`^/uploads/receipts/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp|gif)$`,
)

// IsStoredReceiptPath reports whether raw is a path this API could have
// produced for an uploaded receipt. It is the gate for any client-supplied
// image reference that ends up rendered to a centre admin.
func IsStoredReceiptPath(raw string) bool {
	return storedReceiptPathRe.MatchString(raw)
}
