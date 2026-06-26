package backup

import (
	"os"
	"strings"
)

func InspectBackup(path string) (*InspectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	headers, err := readAllHeaders(path)
	if err != nil {
		return nil, err
	}

	clusters, fleetIndex, err := buildInventory(path, headers)
	if err != nil {
		return nil, err
	}

	mgmt := make(map[string]struct{}, len(clusters))
	for cid := range clusters {
		mgmt[cid] = struct{}{}
	}

	ghosts := make(map[string]int)
	localArtifacts := 0
	fleetDefault := 0

	for _, hdr := range headers {
		name := hdr.Name
		if strings.HasPrefix(name, "clusters.fleet.cattle.io#v1alpha1/fleet-default/") && strings.HasSuffix(name, ".json") {
			fleetDefault++
		}
		if _, ok := LocalExactPaths[name]; ok {
			localArtifacts++
			continue
		}
		if name == "authconfigs.management.cattle.io#v3/local.json" {
			continue
		}
		for _, sub := range LocalSubstrings {
			if strings.Contains(name, sub) {
				localArtifacts++
				break
			}
		}
		for _, match := range clusterIDRe.FindAllString(name, -1) {
			if match == "local" {
				continue
			}
			if _, ok := mgmt[match]; !ok {
				ghosts[match]++
			}
		}
	}

	return &InspectResult{
		Path:           path,
		MemberCount:    len(headers),
		InputSize:      info.Size(),
		Clusters:       clusters,
		FleetMappings:  len(fleetIndex.NameToCluster),
		GhostIDs:       ghosts,
		FleetDefault:   fleetDefault,
		LocalArtifacts: localArtifacts,
	}, nil
}
