package pluginapi

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func validateSemver(v string) error {
	n := normalizeSemver(v)
	if !semver.IsValid(n) {
		return fmt.Errorf("invalid semver %q", v)
	}
	return nil
}

// SatisfiesEngineRange reports whether engineVersion matches rangeExpr.
// rangeExpr is a space-separated list of constraints using the operators
// >=, >, <=, <, = (or bare version meaning =). Example: ">=1.0.0 <2.0.0".
func SatisfiesEngineRange(engineVersion, rangeExpr string) (bool, error) {
	eng := normalizeSemver(engineVersion)
	if !semver.IsValid(eng) {
		return false, fmt.Errorf("invalid engine version %q", engineVersion)
	}
	rangeExpr = strings.TrimSpace(rangeExpr)
	if rangeExpr == "" {
		rangeExpr = DefaultEngineRange
	}
	for _, tok := range strings.Fields(rangeExpr) {
		op, ver, err := parseConstraint(tok)
		if err != nil {
			return false, err
		}
		cmp := semver.Compare(eng, ver)
		switch op {
		case ">=":
			if cmp < 0 {
				return false, nil
			}
		case ">":
			if cmp <= 0 {
				return false, nil
			}
		case "<=":
			if cmp > 0 {
				return false, nil
			}
		case "<":
			if cmp >= 0 {
				return false, nil
			}
		case "=":
			if cmp != 0 {
				return false, nil
			}
		default:
			return false, fmt.Errorf("unknown operator %q", op)
		}
	}
	return true, nil
}

func parseConstraint(tok string) (op, ver string, err error) {
	for _, candidate := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(tok, candidate) {
			raw := strings.TrimSpace(tok[len(candidate):])
			n := normalizeSemver(raw)
			if !semver.IsValid(n) {
				return "", "", fmt.Errorf("invalid constraint version in %q", tok)
			}
			return candidate, n, nil
		}
	}
	n := normalizeSemver(tok)
	if !semver.IsValid(n) {
		return "", "", fmt.Errorf("invalid constraint %q", tok)
	}
	return "=", n, nil
}
