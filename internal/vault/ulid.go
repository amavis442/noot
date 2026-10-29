package vault

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// A note id is a ULID: 48 bits of millisecond timestamp and 80 random bits,
// written as 26 characters of Crockford base32. Ids are unique without having
// to look at the rest of the vault, and still sort by creation time.
const (
	idLen     = 26
	crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // no I, L, O or U
)

// NewID returns a ULID for a note created at now.
func NewID(now time.Time) (string, error) {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], uint64(now.UnixMilli())<<16)
	if _, err := rand.Read(b[6:]); err != nil {
		return "", err
	}
	// Read the 128 bits as one big-endian number and take five bits at a
	// time from the back; the first character is left with the top three.
	hi := binary.BigEndian.Uint64(b[:8])
	lo := binary.BigEndian.Uint64(b[8:])
	out := make([]byte, idLen)
	for i := idLen - 1; i >= 0; i-- {
		out[i] = crockford[lo&0x1f]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out), nil
}
