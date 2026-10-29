package agent

import (
	"regexp"
	"strings"
)

var versionRE = regexp.MustCompile(`(?i)(?:版本|version|v)\s*[:：]?\s*([0-9]+(?:\.[0-9]+){1,3})`)

func VersionConflict(answer, active string, evidence []string) bool {
	if strings.TrimSpace(active) == "" {
		return false
	}
	active = strings.TrimPrefix(strings.TrimSpace(active), "v")
	for _, m := range versionRE.FindAllStringSubmatch(answer, -1) {
		if len(m) > 1 && m[1] != active {
			return true
		}
	}
	for _, e := range evidence {
		for _, m := range versionRE.FindAllStringSubmatch(e, -1) {
			if len(m) > 1 && m[1] != active {
				return true
			}
		}
	}
	return false
}

type VersionGuard struct{ ActiveVersion string }

func (v VersionGuard) Validate(answer string, evidence []string) bool {
	return VersionConflict(answer, v.ActiveVersion, evidence)
}
