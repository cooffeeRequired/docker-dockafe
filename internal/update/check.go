package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	Owner     = "cooffeeRequired"
	Repo      = "docker-dockafe"
	AssetName = "dockafe"
	apiURL    = "https://api.github.com/repos/" + Owner + "/" + Repo + "/releases/latest"
)

// Info describes a newer release available on GitHub.
type Info struct {
	Current        string
	Latest         string
	Tag            string
	URL            string
	AssetURL       string
	AssetName      string
	ChecksumURL    string
	ExpectedSHA256 string
	Available      bool
	CheckedAt      time.Time
	CheckError     string
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckLatest queries GitHub Releases for a newer version than current.
// Network failures return Info with CheckError set (not a Go error), so the TUI can stay quiet.
func CheckLatest(ctx context.Context, current string, client *http.Client) Info {
	info := Info{
		Current:   current,
		CheckedAt: time.Now(),
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		info.CheckError = err.Error()
		return info
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dockafe/"+Normalize(current))

	resp, err := client.Do(req)
	if err != nil {
		info.CheckError = err.Error()
		return info
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		info.CheckError = err.Error()
		return info
	}
	if resp.StatusCode != http.StatusOK {
		info.CheckError = fmt.Sprintf("github api %s", resp.Status)
		return info
	}

	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		info.CheckError = err.Error()
		return info
	}
	if rel.TagName == "" {
		info.CheckError = "empty release tag"
		return info
	}

	info.Tag = rel.TagName
	info.Latest = Normalize(rel.TagName)
	info.URL = rel.HTMLURL
	info.AssetName = AssetName

	var checksumURL, sumsURL string
	for _, a := range rel.Assets {
		switch {
		case a.Name == AssetName && a.BrowserDownloadURL != "":
			info.AssetURL = a.BrowserDownloadURL
		case a.Name == ChecksumAssetName && a.BrowserDownloadURL != "":
			checksumURL = a.BrowserDownloadURL
		case strings.EqualFold(a.Name, "SHA256SUMS") && a.BrowserDownloadURL != "":
			sumsURL = a.BrowserDownloadURL
		}
	}
	// Prefer exact name; fall back to first dockafe* binary if needed.
	if info.AssetURL == "" {
		for _, a := range rel.Assets {
			n := strings.ToLower(a.Name)
			if strings.Contains(n, "dockafe") && !strings.Contains(n, "sha256") && a.BrowserDownloadURL != "" {
				info.AssetURL = a.BrowserDownloadURL
				info.AssetName = a.Name
				break
			}
		}
	}

	info.ChecksumURL = checksumURL
	if info.ChecksumURL == "" {
		info.ChecksumURL = sumsURL
	}
	if info.ChecksumURL != "" {
		sum, err := fetchExpectedSHA256(ctx, client, info.ChecksumURL, info.AssetName)
		if err != nil {
			info.CheckError = "checksum: " + err.Error()
		} else {
			info.ExpectedSHA256 = sum
		}
	}

	if Compare(current, rel.TagName) < 0 {
		info.Available = true
	}
	return info
}

func fetchExpectedSHA256(ctx context.Context, client *http.Client, url, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "dockafe-updater")
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksum: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return ParseSHA256SumFile(string(body), assetName)
}
