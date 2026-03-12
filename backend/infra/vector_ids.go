package infra

import (
	"fmt"

	"github.com/google/uuid"
)

func vectorPointID(versionID string, idx int) string {
	key := fmt.Sprintf("%s_%d", versionID, idx)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(key)).String()
}
