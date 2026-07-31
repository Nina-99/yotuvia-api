package utils

import (
	"crypto/rand"
	"math/big"
)

const referralCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I,O,0,1 to avoid confusion

// GenerateReferralCode creates a unique 6-char alphanumeric code for a beauty center.
func GenerateReferralCode() (string, error) {
	code := make([]byte, 6)
	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(referralCharset))))
		if err != nil {
			return "", err
		}
		code[i] = referralCharset[num.Int64()]
	}
	return string(code), nil
}
