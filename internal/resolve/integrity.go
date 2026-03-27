// SPDX-License-Identifier: MIT

package resolve

import (
	"crypto/sha256"
	"fmt"
)

// HashBytes computes the SHA256 hash of data and returns it as "sha256:<hex>".
// This is the canonical hash format used in lockfile entries.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}
