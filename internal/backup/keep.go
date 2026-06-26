package backup

// keepSetFromOpts returns the set of downstream cluster IDs to retain.
// KeepCluster and KeepClusters are merged for backward compatibility.
func keepSetFromOpts(opts Options) map[string]struct{} {
	set := make(map[string]struct{})
	if opts.KeepCluster != "" {
		set[opts.KeepCluster] = struct{}{}
	}
	for _, cid := range opts.KeepClusters {
		if cid != "" {
			set[cid] = struct{}{}
		}
	}
	return set
}
