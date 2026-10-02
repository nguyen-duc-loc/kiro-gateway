package credentials

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/jsonobject"
)

const selectedProfileKey = "api.codewhisperer.profile"

// Profile errors are fixed categories without account metadata.
var (
	ErrProfileInvalid     = errors.New("profile_invalid")
	ErrProfileUnsupported = errors.New("profile_unsupported")
)

type profileSnapshot struct {
	arn    string
	region string
	digest [sha256.Size]byte
}

// ProfileSnapshot holds a token and selected profile from one SQLite snapshot.
// It is for evidenced inference adapters. Never log or persist its accessors.
type ProfileSnapshot struct {
	credential Snapshot
	profile    profileSnapshot
}

// Credential returns the token snapshot validated against the saved reference.
func (s ProfileSnapshot) Credential() Snapshot { return s.credential }

// ProfileARN returns the selected ARN, for an evidenced wire field only.
func (s ProfileSnapshot) ProfileARN() string { return s.profile.arn }

// ProfileRegion returns the region from the ARN, not the Identity Center region.
func (s ProfileSnapshot) ProfileRegion() string { return s.profile.region }

// ProfileDigest returns the digest of exact source bytes for an in memory pin.
func (s ProfileSnapshot) ProfileDigest() [sha256.Size]byte { return s.profile.digest }

// String prevents ordinary formatting from revealing account data.
func (s ProfileSnapshot) String() string { return "[selected profile snapshot]" }

// GoString also protects Go syntax formatting.
func (s ProfileSnapshot) GoString() string { return s.String() }

// ReadProfileSnapshot reads the two fixed records in one transaction, checking
// the token against expected before validating the profile. It does not change
// the saved reference or keep a profile pin between calls.
func (r Reader) ReadProfileSnapshot(ctx context.Context, expected config.Session) (ProfileSnapshot, error) {
	d := config.Default()
	d.Session = &expected
	if d.Validate() != nil {
		return ProfileSnapshot{}, ErrChanged
	}
	captured, err := r.readSelected(ctx, &expected)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	return ProfileSnapshot{credential: captured.snapshot, profile: captured.profile}, nil
}

func parseProfile(data []byte) (profileSnapshot, error) {
	o, err := jsonobject.Parse(data, config.MaxBytes)
	if err != nil {
		return profileSnapshot{}, ErrProfileInvalid
	}
	var arn string
	if json.Unmarshal(o["arn"], &arn) != nil || !config.VisibleASCII(arn, 2048) {
		return profileSnapshot{}, ErrProfileInvalid
	}
	camel, hasCamel := o["profileName"]
	snake, hasSnake := o["profile_name"]
	if hasCamel == hasSnake {
		return profileSnapshot{}, ErrProfileInvalid
	}
	name := camel
	if hasSnake {
		name = snake
	}
	// A pointer distinguishes JSON null from the permitted empty string. The
	// decoded name is discarded and never becomes a routing or identity input.
	var decodedName *string
	if json.Unmarshal(name, &decodedName) != nil || decodedName == nil {
		return profileSnapshot{}, ErrProfileInvalid
	}
	parts := strings.Split(arn, ":")
	if len(parts) != 6 || parts[0] != "arn" || parts[1] != "aws" || parts[2] != "codewhisperer" ||
		(parts[3] != "us-east-1" && parts[3] != "eu-central-1") || len(parts[4]) != 12 ||
		!profileCharacters(parts[4], false) || !strings.HasPrefix(parts[5], "profile/") {
		return profileSnapshot{}, ErrProfileUnsupported
	}
	id := strings.TrimPrefix(parts[5], "profile/")
	if len(id) < 1 || len(id) > 128 || !profileCharacters(id, true) {
		return profileSnapshot{}, ErrProfileUnsupported
	}
	h := sha256.New()
	h.Write([]byte("kiro-gateway/probe-profile-v1\x00"))
	h.Write(data)
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return profileSnapshot{arn: arn, region: parts[3], digest: digest}, nil
}

func profileCharacters(value string, resource bool) bool {
	for _, c := range value {
		if c >= '0' && c <= '9' {
			continue
		}
		if resource && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '-') {
			continue
		}
		return false
	}
	return true
}
