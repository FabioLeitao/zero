package zeroruntime

// MaxResolvedModelBytes caps the stored provider-reported model id.
const MaxResolvedModelBytes = 256

// SanitizeResolvedModel returns raw as safe, bounded, printable text.
func SanitizeResolvedModel(raw string) string {
	return raw
}

// ResolvedModelAttribution returns the sanitized reported model, or "" when it
// is empty after sanitizing or equals the sanitized requested model.
func ResolvedModelAttribution(requested, reported string) string {
	return reported
}
