package update

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const ChecksumAssetName = "dockafe.sha256"

// ParseSHA256SumFile extracts the hex digest for assetName from sha256sum-style text.
func ParseSHA256SumFile(content, assetName string) (string, error) {
	assetName = strings.TrimSpace(assetName)
	if assetName == "" {
		assetName = AssetName
	}
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		name := fields[1]
		name = strings.TrimPrefix(name, "*") // binary mode marker
		if name == assetName || name == "./"+assetName {
			if len(sum) != 64 {
				return "", fmt.Errorf("invalid sha256 length for %s", assetName)
			}
			if _, err := hex.DecodeString(sum); err != nil {
				return "", fmt.Errorf("invalid sha256 hex: %w", err)
			}
			return sum, nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %q not found", assetName)
}

// HashReader returns lowercase hex SHA-256 of r.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
