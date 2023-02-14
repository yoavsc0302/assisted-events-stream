package process

import (
	"strings"

	"github.com/openshift-assisted/assisted-events-streams/internal/types"
)

func getHostsFromEvent(event *types.EnrichedEvent) []map[string]interface{} {
	hosts := []map[string]interface{}{}
	if clusterHosts, ok := event.Cluster["hosts"]; ok {
		if hs, ok := clusterHosts.([]interface{}); ok {
			for _, h := range hs {
				if host, ok := h.(map[string]interface{}); ok {
					hosts = append(hosts, host)
				}
			}
		}
	}
	return hosts
}

func getInfraEnvFromHost(host map[string]interface{}) (map[string]interface{}, bool) {
	if infra, ok := host["infra_env"]; ok {
		infraEnv, ok := infra.(map[string]interface{})
		return infraEnv, ok
	}
	return map[string]interface{}{}, false
}

func normalizeInfraEnvStats(stats *types.HostsInfraEnvSummary, total int) {
	// k being stat label, v map with value as key and count as value
	for k, v := range *stats {
		// j is the stat's value, val the number of occurrences
		for j, val := range v {
			(*stats)[k][j] = val / float64(total)
		}
	}
}

func updateInfraEnvStats(stats *types.HostsInfraEnvSummary, infraEnv map[string]interface{}) {
	for k, _ := range *stats {
		if s, ok := infraEnv[k]; ok {
			if label, ok := s.(string); ok {
				// avoid ambiguous values/labels (i.e. 4.12)
				label = strings.Replace(label, ".", "_", -1)
				(*stats)[k][label] = (*stats)[k][label] + 1
			}
		}
	}
}

func AddHostsSummary(event *types.EnrichedEvent) error {
	hostsSummary := &types.HostsSummary{}
	totalInfraEnvs := 0
	stats := types.HostsInfraEnvSummary{
		"type":              map[string]float64{},
		"cpu_architecture":  map[string]float64{},
		"openshift_version": map[string]float64{},
	}

	hosts := getHostsFromEvent(event)

	for _, host := range hosts {
		hostsSummary.HostCount += 1
		if infraEnv, ok := getInfraEnvFromHost(host); ok {
			totalInfraEnvs += 1
			updateInfraEnvStats(&stats, infraEnv)
		}
	}

	normalizeInfraEnvStats(&stats, totalInfraEnvs)

	k, _ := getHighestKeyValue(stats["type"])
	hostsSummary.IsoType = k

	_, v := getHighestKeyValue(stats["cpu_architecture"])
	if v < 1.0 {
		hostsSummary.HasEtherogeneousArchitecture = true
	}

	hostsSummary.InfraEnv = stats
	event.HostsSummary = hostsSummary

	return nil
}

func getHighestKeyValue(stats map[string]float64) (string, float64) {
	key := ""
	value := 0.0

	for k, v := range stats {
		if v > value {
			value = v
			key = k
		}
	}
	return key, value
}
