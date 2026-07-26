package update

import (
	"strconv"
	"strings"
)

// Normalize strips a leading "v" and any git-describe suffix ("-N-gHASH", "-dirty").
func Normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// Compare returns -1 if a < b, 0 if equal, 1 if a > b (semver major.minor.patch).
// Non-numeric parts are treated as 0.
func Compare(a, b string) int {
	ap := parseParts(Normalize(a))
	bp := parseParts(Normalize(b))
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		avar, bvar := 0, 0
		if i < len(ap) {
			avar = ap[i]
		}
		if i < len(bp) {
			bvar = bp[i]
		}
		if avar < bvar {
			return -1
		}
		if avar > bvar {
			return 1
		}
	}
	return 0
}

func parseParts(v string) []int {
	if v == "" {
		return []int{0}
	}
	bits := strings.Split(v, ".")
	out := make([]int, 0, len(bits))
	for _, bit := range bits {
		n, err := strconv.Atoi(bit)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}
