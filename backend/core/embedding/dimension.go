package embedding

import "fmt"

// ExpectedDimensions returns the fixed embedding width for known models.
func ExpectedDimensions(model string) (int, error) {
	switch model {
	case "text-embedding-3-small", "text-embedding-ada-002":
		return 1536, nil
	case "text-embedding-3-large":
		return 3072, nil
	default:
		return 0, fmt.Errorf("unknown embedding model dimension: %s", model)
	}
}
