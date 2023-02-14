package process

import (
	"io/ioutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Process message", func() {
	var (
		logger   *logrus.Logger
		enricher *EventEnricher
	)
	BeforeEach(func() {
		logger = logrus.New()
		logger.Out = ioutil.Discard
		enricher = NewEventEnricher(logger)
	})
	When("Enrich a cluster event with fields to be transformed", func() {
		It("gets transformed correctly", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"feature_usage": `{"Cluster Tags":{"id":"CLUSTER_TAGS","name":"Cluster Tags"},"Hyperthreading":{"data":{"hyperthreading_enabled":"all"},"id":"HYPERTHREADING","name":"Hyperthreading"},"SDN network type":{"id":"SDN_NETWORK_TYPE","name":"SDN network type"}}`,
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)

			Expect(enrichedEvent.Message).Should(Equal(message))
			featureUsage, ok := enrichedEvent.Cluster["feature_usage"]
			Expect(ok).To(BeTrue())

			featureList, ok := featureUsage.([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(featureList)).To(Equal(3))

			ids := make([]string, 0)
			for _, m := range featureList {
				feature, ok := m.(map[string]interface{})
				Expect(ok).To(BeTrue())
				id, ok := feature["id"].(string)
				Expect(ok).To(BeTrue())
				ids = append(ids, id)
			}
			Expect(ids).To(ContainElements("HYPERTHREADING", "CLUSTER_TAGS", "SDN_NETWORK_TYPE"))
		})
	})
	When("Enrich a cluster event with deep fields to be unpacked", func() {
		It("gets unpacked correctly", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"pull_secret": "alotoftext",
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)

			Expect(enrichedEvent.Message).Should(Equal(message))
			clusterHosts, ok := enrichedEvent.Cluster["hosts"].([]interface{})
			Expect(ok).To(BeTrue())

			for _, host := range clusterHosts {
				h, ok := host.(map[string]interface{})
				Expect(ok).To(BeTrue())

				inventory, ok := h["inventory"]
				Expect(ok).To(BeTrue())

				i, ok := inventory.(map[string]interface{})
				Expect(ok).To(BeTrue())

				cpu, ok := i["cpu"]
				Expect(ok).To(BeTrue())

				c, ok := cpu.(map[string]interface{})
				Expect(ok).To(BeTrue())

				architecture, ok := c["architecture"]
				Expect(architecture).To(Equal("x86_64"))
			}
		})
	})
	When("Enrich a cluster event with infraenv to be embedded", func() {
		It("infraenv embedded correctly", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"pull_secret": "alotoftext",
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)
			clusterHosts, ok := enrichedEvent.Cluster["hosts"]
			Expect(ok).To(BeTrue())

			cHosts, ok := clusterHosts.([]map[string]interface{})
			for _, host := range cHosts {
				Expect(ok).To(BeTrue())
				hostInfraEnvID, ok := host["infra_env_id"]
				Expect(ok).To(BeTrue())
				infraEnv, ok := host["infra_env"]
				Expect(ok).To(BeTrue())
				tInfraEnv, ok := infraEnv.(map[string]interface{})
				Expect(ok).To(BeTrue())
				infraEnvID, ok := tInfraEnv["id"]
				Expect(ok).To(BeTrue())
				Expect(hostInfraEnvID).To(Equal(infraEnvID))
			}
		})
	})
	When("Enrich a cluster event with fields to be deleted", func() {
		It("gets deleted correctly", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"pull_secret": "alotoftext",
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)

			Expect(enrichedEvent.Message).Should(Equal(message))
			_, ok := enrichedEvent.Cluster["pull_secret"]
			Expect(ok).To(BeFalse())
		})
	})
	When("Enrich a cluster event with more fields to be deleted", func() {
		It("gets deleted correctly", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"kind":           "Cluster",
				"pull_secret":    "alotoftext",
				"ssh_public_key": "mykey",
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)

			Expect(enrichedEvent.Message).Should(Equal(message))

			_, ok := enrichedEvent.Cluster["pull_secret"]
			Expect(ok).To(BeFalse())

			_, ok = enrichedEvent.Cluster["kind"]
			Expect(ok).To(BeFalse())

			_, ok = enrichedEvent.Cluster["ssh_public_key"]
			Expect(ok).To(BeFalse())

		})
	})
	When("Enrich a cluster event with deep fields to be deleted", func() {
		It("gets deleted correctly", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"kind":           "Cluster",
				"pull_secret":    "alotoftext",
				"ssh_public_key": "mykey",
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)

			Expect(enrichedEvent.Message).Should(Equal(message))

			for _, infraEnv := range enrichedEvent.InfraEnvs {

				_, ok := infraEnv["ssh_authorized_key"]
				Expect(ok).To(BeFalse())
			}
		})
	})
	When("Enrich a cluster event with fields to be anonymized", func() {
		It("gets anonymized correctly", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"pull_secret": "alotoftext",
				"user_name":   "my sensitive data",
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)

			Expect(enrichedEvent.Message).Should(Equal(message))
			_, ok := enrichedEvent.Cluster["user_name"]
			Expect(ok).To(BeFalse())

			userId, ok := enrichedEvent.Cluster["user_id"]
			Expect(ok).To(BeTrue())

			// b3ce829e4327ba06e2ce8bd976ced308 is md5 for "my sensitive data"
			expectedHash := "b3ce829e4327ba06e2ce8bd976ced308"
			Expect(userId).To(Equal(expectedHash))

			clusterHosts, ok := enrichedEvent.Cluster["hosts"].([]interface{})
			Expect(ok).To(BeTrue())

			for _, host := range clusterHosts {
				h, ok := host.(map[string]interface{})
				Expect(ok).To(BeTrue())

				_, ok = h["user_name"]
				Expect(ok).To(BeFalse())

				userId, ok = h["user_id"]
				Expect(ok).To(BeTrue())
				Expect(userId).To(Equal(expectedHash))
			}
		})
	})
	When("Enrich a cluster event with fields to be summarized", func() {
		It("gets correct summary", func() {
			message := "Cluster with ID myid updated"
			event := getEvent("cluster_updated", message)
			cluster := map[string]interface{}{
				"pull_secret": "alotoftext",
				"user_name":   "my sensitive data",
			}
			hosts := getHosts()
			infraEnvs := getInfraEnvs()
			enrichedEvent := enricher.GetEnrichedEvent(event, cluster, hosts, infraEnvs)
			assertHostsSummary(enrichedEvent)
		})
	})
})

func getEvent(name, message string) *types.Event {
	metadata := map[string]interface{}{
		"foo": "bar",
	}
	payload := map[string]interface{}{
		"name":       name,
		"message":    message,
		"event_time": "2023-01-27T03:40:08.998Z",
	}
	return &types.Event{
		Name:     "Foobar",
		Payload:  payload,
		Metadata: metadata,
	}
}

func getHosts() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":           "5d8054e5-6d52-4bdb-8629-65c258d29de9",
			"foo":          "foobar",
			"infra_env_id": "94b5c5b6-f798-41cc-9187-ffe14df68d48",
			"user_name":    "my sensitive data",
			"inventory":    `{"bmc_address":"0.0.0.0","bmc_v6address":"::/0","boot":{"current_boot_mode":"bios"},"cpu":{"architecture":"x86_64","count":16,"flags":["fpu","vme","de","pse","tsc","msr","pae","mce","cx8","apic","sep","mtrr","pge","mca","cmov","pat","pse36","clflush","mmx","fxsr","sse","sse2","ss","syscall","nx","pdpe1gb","rdtscp","lm","constant_tsc","arch_perfmon","rep_good","nopl","xtopology","cpuid","tsc_known_freq","pni","pclmulqdq","vmx","ssse3","fma","cx16","pdcm","pcid","sse4_1","sse4_2","x2apic","movbe","popcnt","tsc_deadline_timer","aes","xsave","avx","f16c","rdrand","hypervisor","lahf_lm","abm","3dnowprefetch","cpuid_fault","invpcid_single","ssbd","ibrs","ibpb","stibp","ibrs_enhanced","tpr_shadow","vnmi","flexpriority","ept","vpid","ept_ad","fsgsbase","tsc_adjust","bmi1","avx2","smep","bmi2","erms","invpcid","mpx","avx512f","avx512dq","rdseed","adx","smap","clflushopt","clwb","avx512cd","avx512bw","avx512vl","xsaveopt","xsavec","xgetbv1","xsaves","arat","umip","pku","ospke","avx512_vnni","md_clear","arch_capabilities"],"frequency":2095.076,"model_name":"Intel(R) Xeon(R) Gold 5218R CPU @ 2.10GHz"},"disks":[{"by_id":"/dev/disk/by-id/wwn-0x05abcdb535529a2a","by_path":"/dev/disk/by-path/pci-0000:00:04.0-scsi-0:0:0:0","drive_type":"HDD","has_uuid":true,"hctl":"0:0:0:0","id":"/dev/disk/by-id/wwn-0x05abcdb535529a2a","installation_eligibility":{"eligible":true,"not_eligible_reasons":null},"model":"QEMU_HARDDISK","name":"sda","path":"/dev/sda","serial":"05abcdb535529a2a","size_bytes":141733920768,"smart":"SMART support is:     Unavailable - device lacks SMART capability.\n","vendor":"QEMU","wwn":"0x05abcdb535529a2a"},{"bootable":true,"by_path":"/dev/disk/by-path/pci-0000:00:04.0-scsi-0:0:0:3","drive_type":"ODD","has_uuid":true,"hctl":"0:0:0:3","id":"/dev/disk/by-path/pci-0000:00:04.0-scsi-0:0:0:3","installation_eligibility":{"not_eligible_reasons":["Disk is removable","Disk is too small (disk only has 1.2 GB, but 20 GB are required)","Drive type is ODD, it must be one of HDD, SSD, Multipath."]},"is_installation_media":true,"model":"QEMU_CD-ROM","name":"sr0","path":"/dev/sr0","removable":true,"serial":"drive-scsi0-0-0-3","size_bytes":1212153856,"smart":"SMART support is:     Unavailable - device lacks SMART capability.\n","vendor":"QEMU"}],"gpus":[{"address":"0000:00:02.0"}],"hostname":"master-0-0","interfaces":[{"flags":["up","broadcast","multicast"],"has_carrier":true,"ipv4_addresses":["192.168.123.55/24"],"ipv6_addresses":[],"mac_address":"52:54:00:f2:cb:55","mtu":1500,"name":"ens3","product":"0x0001","speed_mbps":-1,"type":"physical","vendor":"0x1af4"}],"memory":{"physical_bytes":34359738368,"physical_bytes_method":"dmidecode","usable_bytes":33706188800},"routes":[{"destination":"0.0.0.0","family":2,"gateway":"192.168.123.1","interface":"ens3","metric":100},{"destination":"10.88.0.0","family":2,"interface":"cni-podman0"},{"destination":"192.168.123.0","family":2,"interface":"ens3","metric":100},{"destination":"::1","family":10,"interface":"lo","metric":256},{"destination":"fe80::","family":10,"interface":"cni-podman0","metric":256},{"destination":"fe80::","family":10,"interface":"ens3","metric":1024}],"system_vendor":{"manufacturer":"Red Hat","product_name":"KVM","virtual":true},"tpm_version":"none"}`,
		},
		{
			"id":           "9024b650-f31f-420b-b279-3296a6e7d2cc",
			"foo":          "foobar",
			"infra_env_id": "177880c8-7c2c-49d3-b7c2-42625c31eb05",
			"user_name":    "my sensitive data",
			"inventory":    `{"bmc_address":"0.0.0.0","bmc_v6address":"::/0","boot":{"current_boot_mode":"bios"},"cpu":{"architecture":"x86_64","count":16,"flags":["fpu","vme","de","pse","tsc","msr","pae","mce","cx8","apic","sep","mtrr","pge","mca","cmov","pat","pse36","clflush","mmx","fxsr","sse","sse2","ss","syscall","nx","pdpe1gb","rdtscp","lm","constant_tsc","arch_perfmon","rep_good","nopl","xtopology","cpuid","tsc_known_freq","pni","pclmulqdq","vmx","ssse3","fma","cx16","pdcm","pcid","sse4_1","sse4_2","x2apic","movbe","popcnt","tsc_deadline_timer","aes","xsave","avx","f16c","rdrand","hypervisor","lahf_lm","abm","3dnowprefetch","cpuid_fault","invpcid_single","ssbd","ibrs","ibpb","stibp","ibrs_enhanced","tpr_shadow","vnmi","flexpriority","ept","vpid","ept_ad","fsgsbase","tsc_adjust","bmi1","avx2","smep","bmi2","erms","invpcid","mpx","avx512f","avx512dq","rdseed","adx","smap","clflushopt","clwb","avx512cd","avx512bw","avx512vl","xsaveopt","xsavec","xgetbv1","xsaves","arat","umip","pku","ospke","avx512_vnni","md_clear","arch_capabilities"],"frequency":2095.076,"model_name":"Intel(R) Xeon(R) Gold 5218R CPU @ 2.10GHz"},"disks":[{"by_id":"/dev/disk/by-id/wwn-0x05abcdb535529a2a","by_path":"/dev/disk/by-path/pci-0000:00:04.0-scsi-0:0:0:0","drive_type":"HDD","has_uuid":true,"hctl":"0:0:0:0","id":"/dev/disk/by-id/wwn-0x05abcdb535529a2a","installation_eligibility":{"eligible":true,"not_eligible_reasons":null},"model":"QEMU_HARDDISK","name":"sda","path":"/dev/sda","serial":"05abcdb535529a2a","size_bytes":141733920768,"smart":"SMART support is:     Unavailable - device lacks SMART capability.\n","vendor":"QEMU","wwn":"0x05abcdb535529a2a"},{"bootable":true,"by_path":"/dev/disk/by-path/pci-0000:00:04.0-scsi-0:0:0:3","drive_type":"ODD","has_uuid":true,"hctl":"0:0:0:3","id":"/dev/disk/by-path/pci-0000:00:04.0-scsi-0:0:0:3","installation_eligibility":{"not_eligible_reasons":["Disk is removable","Disk is too small (disk only has 1.2 GB, but 20 GB are required)","Drive type is ODD, it must be one of HDD, SSD, Multipath."]},"is_installation_media":true,"model":"QEMU_CD-ROM","name":"sr0","path":"/dev/sr0","removable":true,"serial":"drive-scsi0-0-0-3","size_bytes":1212153856,"smart":"SMART support is:     Unavailable - device lacks SMART capability.\n","vendor":"QEMU"}],"gpus":[{"address":"0000:00:02.0"}],"hostname":"master-0-0","interfaces":[{"flags":["up","broadcast","multicast"],"has_carrier":true,"ipv4_addresses":["192.168.123.55/24"],"ipv6_addresses":[],"mac_address":"52:54:00:f2:cb:55","mtu":1500,"name":"ens3","product":"0x0001","speed_mbps":-1,"type":"physical","vendor":"0x1af4"}],"memory":{"physical_bytes":34359738368,"physical_bytes_method":"dmidecode","usable_bytes":33706188800},"routes":[{"destination":"0.0.0.0","family":2,"gateway":"192.168.123.1","interface":"ens3","metric":100},{"destination":"10.88.0.0","family":2,"interface":"cni-podman0"},{"destination":"192.168.123.0","family":2,"interface":"ens3","metric":100},{"destination":"::1","family":10,"interface":"lo","metric":256},{"destination":"fe80::","family":10,"interface":"cni-podman0","metric":256},{"destination":"fe80::","family":10,"interface":"ens3","metric":1024}],"system_vendor":{"manufacturer":"Red Hat","product_name":"KVM","virtual":true},"tpm_version":"none"}`,
		},
	}
}

func getInfraEnvs() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":                 "94b5c5b6-f798-41cc-9187-ffe14df68d48",
			"foo":                "foobar",
			"ssh_authorized_key": "mykey",
			"type":               "full-iso",
			"cpu_architecture":   "x86",
			"openshift_version":  "4.12",
		},
		{
			"id":                 "177880c8-7c2c-49d3-b7c2-42625c31eb05",
			"foo":                "foobar",
			"ssh_authorized_key": "yourkey",
			"type":               "full-iso",
			"cpu_architecture":   "arm64",
			"openshift_version":  "4.12",
		},
	}
}
