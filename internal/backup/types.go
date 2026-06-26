package backup

import "time"

type ClusterMeta struct {
	DisplayName string
	Kind        string
	Internal    bool
}

type FleetIndex struct {
	NameToCluster    map[string]string
	DisplayToCluster map[string]string
}

func NewFleetIndex() *FleetIndex {
	return &FleetIndex{
		NameToCluster:    make(map[string]string),
		DisplayToCluster: make(map[string]string),
	}
}

type Options struct {
	Input          string
	Output         string
	Report         string
	KeepCluster    string
	KeepRKE1Only   bool
	RemoveClusters []string
	NoAutoOrphans  bool
	CompressLevel  int
	Fast           bool
	Quiet          bool
	Verbose        bool
	LogFile        string
	InspectOnly    bool
	ProgressFn     func(current, total int)
}

type Result struct {
	InputPath      string
	OutputPath     string
	InputSize      int64
	OutputSize     int64
	Elapsed        time.Duration
	CompressLevel  int
	Clusters       map[string]ClusterMeta
	FleetMappings  int
	RemoveIDs      map[string]struct{}
	AutoOrphans    []string
	ExplicitRemove []string
	Kept           []string
	Removed        []RemovedEntry
	KeptBytes      int64
}

type RemovedEntry struct {
	Path   string
	Reason string
}

type InspectResult struct {
	Path           string
	MemberCount    int
	InputSize      int64
	Clusters       map[string]ClusterMeta
	FleetMappings  int
	GhostIDs       map[string]int
	FleetDefault   int
	LocalArtifacts int
}
