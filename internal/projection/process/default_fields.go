package process

func getDefaultFieldsToUnpack() []string {
	return []string{
		"cluster.feature_usage",
		"cluster.validations_info",
		"cluster.connectivity_majority_groups",
		"cluster.hosts[*].inventory",
		"cluster.hosts[*].connectivity",
		"cluster.hosts[*].disks_info",
		"cluster.hosts[*].domain_name_resolutions",
		"cluster.hosts[*].free_addresses",
		"cluster.hosts[*].images_status",
		"cluster.hosts[*].ntp_sources",
		"cluster.hosts[*].validations_info",
	}
}

func getDefaultFieldsMapToListDropKey() []string {
	return []string{
		"cluster.feature_usage",
	}
}

func getDefaultFieldsMapToList() []string {
	return []string{
		"cluster.connectivity_majority_groups",
		"cluster.hosts[*].disks_info",
		"cluster.hosts[*].images_status",
	}
}

func getDefaultFieldsToDelete() []string {
	return []string{
		"cluster.pull_secret",
		"cluster.ssh_public_key",
		"cluster.hosts[*].infra_env.ssh_authorized_key",
		"infra_envs[*].ssh_authorized_key",
	}
}

func getDefaultFieldsToAnonymize() map[string]string {
	return map[string]string{
		"cluster.user_name":                    "cluster.user_id",
		"cluster.hosts[*].user_name":           "cluster.hosts[*].user_id",
		"cluster.hosts[*].infra_env.user_name": "cluster.hosts[*].infra_env.user_id",
	}
}
