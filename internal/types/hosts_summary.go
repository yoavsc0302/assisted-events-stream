package types

type HostsSummary struct {
	HasEtherogeneousArchitecture bool                 `json:"heterogeneous_arch"`
	HostCount                    int                  `json:"host_count"`
	InfraEnv                     HostsInfraEnvSummary `json:"infra_env"`
	IsoType                      string               `json:"iso_type"`
}

/**
 * Example:
 * {
 *   "type": { // in this case cluster has 90% of the host with minimal iso, and one with full-iso
 *      "minimal-iso": 0.9,
 *      "full-iso": 0.1
 *   }
 * }
 */
type HostsInfraEnvSummary map[string]map[string]float64
