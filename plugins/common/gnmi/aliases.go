package gnmi

import (
	"sort"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
)

// Try to find the alias for the given path
type aliasCandidate struct {
	path, alias string
}

func (h *Handler) AddAlias(name, origin, subscriptionPath string) error {
	// Build the subscription path without keys
	path, err := parsePath(origin, subscriptionPath, "")
	if err != nil {
		return err
	}
	info := newInfoFromPathWithoutKeys(path)
	if h.EnforceFirstNamespaceAsOrigin {
		info.enforceFirstNamespaceAsOrigin()
	}

	// If the user didn't provide a measurement name, use last path element
	if name == "" && len(info.segments) > 0 {
		name = info.segments[len(info.segments)-1].id
	}
	if name != "" {
		h.aliases[info] = name
	}
	return nil
}

func (h *Handler) lookupAlias(info *pathInfo) (aliasPath, alias string) {
	candidates := make([]aliasCandidate, 0, len(h.aliases))
	for i, a := range h.aliases {
		if !i.isSubPathOf(info) {
			continue
		}
		candidates = append(candidates, aliasCandidate{i.String(), a})
	}
	if len(candidates) == 0 {
		return "", ""
	}

	// Reverse sort the candidates by path length so we can use the longest match
	sort.SliceStable(candidates, func(i, j int) bool {
		return len(candidates[i].path) > len(candidates[j].path)
	})

	return candidates[0].path, candidates[0].alias
}

func parsePath(origin, pathToParse, target string) (*gnmi.Path, error) {
	gnmiPath, err := xpath.ToGNMIPath(pathToParse)
	if err != nil {
		return nil, err
	}
	gnmiPath.Origin = origin
	gnmiPath.Target = target
	return gnmiPath, err
}
