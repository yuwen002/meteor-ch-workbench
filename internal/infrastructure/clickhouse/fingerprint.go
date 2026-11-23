package clickhouse

import (
	"crypto/sha256"
	"encoding/hex"
)

func configFingerprint(dsn string) string {
	sum:=sha256.Sum256([]byte(dsn))
	return hex.EncodeToString(sum[:])
}
