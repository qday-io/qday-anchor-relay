package crypto

// AssertError identifies an error that indicates an internal code consistency
// issue and should be treated as a critical and unrecoverable error.
type AssertError string

// Error returns the assertion error as a human-readable string and satisfies
// the error interface.
func (e AssertError) Error() string {
	return "assertion failed: " + string(e)
}

var ErrInvalidCryptoScheme = AssertError("unsupported crypto scheme")
var ErrInvalidPrivacyLevel = AssertError("unsupported privacy level")
var ErrMismatchedCryptoSchemePrivacyLevel = AssertError("mismatch crypto scheme and privacy level")
var ErrMismatchedSeedType = AssertError("mismatched seed type")
var ErrCorruptedSeed = AssertError("corrupted seed")
var ErrInvalidAddress = AssertError("invalid address")
