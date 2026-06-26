package backup

const (
	ClusterPrefix      = "clusters.management.cattle.io#v3/"
	FleetClusterPrefix = "clusters.fleet.cattle.io#v1alpha1/fleet-default/"
	LocalReason        = "local cluster (recreated on restore Rancher)"
)

var LocalExactPaths = map[string]struct{}{
	"clusters.management.cattle.io#v3/local.json":                                                              {},
	"namespaces.#v1/local.json":                                                                                {},
	"namespaces.#v1/fleet-local.json":                                                                          {},
	"namespaces.#v1/cattle-fleet-local-system.json":                                                            {},
	"fleetworkspaces.management.cattle.io#v3/fleet-local.json":                                                 {},
	"clusterroles.rbac.authorization.k8s.io#v1/local-clusterowner.json":                                        {},
	"clusterrolebindings.rbac.authorization.k8s.io#v1/cattle-fleet-local-system-fleet-agent-role-binding.json": {},
	"clusterroles.rbac.authorization.k8s.io#v1/cattle-fleet-local-system-fleet-agent-role.json":                {},
}

var LocalSubstrings = []string{
	"fleet-local/",
	"fleet-local",
	"#v3/local/",
	"roles.rbac.authorization.k8s.io#v1/local/",
	"rolebindings.rbac.authorization.k8s.io#v1/local/",
	"roles.rbac.authorization.k8s.io#v1/fleet-local/",
	"rolebindings.rbac.authorization.k8s.io#v1/fleet-local/",
	"configmaps.#v1/cattle-fleet-local-system/",
	"deployments.apps#v1/cattle-fleet-local-system/",
	"configmaps.#v1/fleet-local/",
	"nodes.management.cattle.io#v3/local/",
	"projects.management.cattle.io#v3/local/",
	"clusterroletemplatebindings.management.cattle.io#v3/local/",
	"clusterregistrationtokens.management.cattle.io#v3/local/",
	"clusterregistrationtokens.fleet.cattle.io#v1alpha1/fleet-local/",
	"clusters.fleet.cattle.io#v1alpha1/fleet-local/",
	"clusters.provisioning.cattle.io#v1/fleet-local/",
	"bundles.fleet.cattle.io#v1alpha1/fleet-local/",
	"clustergroups.fleet.cattle.io#v1alpha1/fleet-local/",
	"clusterregistrations.fleet.cattle.io#v1alpha1/fleet-local/",
}

var FleetPathPrefixes = []string{
	"clusters.fleet.cattle.io#v1alpha1/fleet-default/",
	"clusters.provisioning.cattle.io#v1/fleet-default/",
	"bundles.fleet.cattle.io#v1alpha1/fleet-default/",
	"clusterregistrationtokens.fleet.cattle.io#v1alpha1/fleet-default/",
}

var JSONClusterPrefixes = []string{
	"clusters.fleet.cattle.io#v1alpha1/",
	"clusters.provisioning.cattle.io#v1/",
	"bundles.fleet.cattle.io#v1alpha1/fleet-default/",
}
