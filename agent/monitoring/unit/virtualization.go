package monitoring

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func Virtualized() string {
	// Prefer systemd-detect-virt if available; fallback to CPUID.
	if out, err := exec.Command("systemd-detect-virt").Output(); err == nil {
		virt := strings.TrimSpace(string(out))
		if virt != "" {
			return virt
		}
	}

	// Non-systemd environments (e.g., Alpine containers): try container heuristics.
	if ct := detectContainer(); ct != "" {
		return ct
	}

	// Fallback (any OS): CPUID hypervisor bit and vendor mapping.
	return detectByCPUID()
}

// detectByCPUID 在不依赖外部指令集大库的情况下，结合 Linux DMI 与内核标记精准判定虚拟化环境
func detectByCPUID() string {
	dmiPaths := []string{
		"/sys/class/dmi/id/sys_vendor",
		"/sys/class/dmi/id/product_name",
		"/sys/class/dmi/id/bios_vendor",
	}
	var dmiCombined string
	for _, p := range dmiPaths {
		if data, err := os.ReadFile(p); err == nil {
			dmiCombined += " " + strings.ToLower(string(data))
		}
	}

	hasHypervisorFlag := false
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		content := string(data)
		if strings.Contains(content, "hypervisor") {
			hasHypervisorFlag = true
		}
	}

	if !hasHypervisorFlag && strings.TrimSpace(dmiCombined) == "" {
		return "none"
	}

	vendorMap := map[string][]string{
		"kvm":        {"kvm", "qemu", "bochs"},
		"vmware":     {"vmware"},
		"virtualbox": {"virtualbox", "innotek", "vbox"},
		"microsoft":  {"microsoft", "hyper-v", "msvm", "mshyperv"},
		"xen":        {"xen"},
		"bhyve":      {"bhyve"},
		"parallels":  {"parallels"},
		"acrn":       {"acrn"},
	}

	for name, keys := range vendorMap {
		for _, key := range keys {
			if strings.Contains(dmiCombined, key) {
				return name
			}
		}
	}

	if hasHypervisorFlag {
		return "kvm" // 现代主流 Linux 云主机缺省大多基于 KVM
	}

	return "none"
}

// detectContainer attempts to detect common Linux container environments when systemd isn't available.
// Returns a systemd-detect-virt-like string such as "docker", "podman", "lxc", "container" or empty if not detected.
func detectContainer() string {
	// Definite file markers first.
	if fileExists("/.dockerenv") {
		return "docker"
	}
	if fileExists("/run/.containerenv") { // podman / CRI-O
		if s := parseCgroupForContainer(); s != "" {
			return s
		}
		return "container"
	}

	// cgroup based detection (safer & more specific than broad substring checks)
	if s := parseCgroupForContainer(); s != "" {
		return s
	}
	if fileExists("/dev/.lxc-boot-id") {
		return "lxc"
	}
	if fileExists("/.komari-agent-container") {
		return "container"
	}
	// (Removed mountinfo heuristics which caused host false positives when Docker/Kube tools are installed.)
	return ""
}

func fileExists(p string) bool {
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return true
	}
	return false
}

// Patterns target leaf elements referencing container IDs instead of any occurrence of runtime name to reduce false positives.
var (
	dockerIDPattern    = regexp.MustCompile(`(?m)/(?:docker|cri-containerd)[/-]([0-9a-f]{12,64})(?:\.scope)?$`)
	dockerScopePattern = regexp.MustCompile(`(?m)/docker-[0-9a-f]{12,64}\.scope$`)
	kubePattern        = regexp.MustCompile(`(?m)/kubepods[/.].*([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}).*`) // pod UID
	podmanPattern      = regexp.MustCompile(`(?m)/(?:libpod|podman)[-_]([0-9a-f]{12,64})(?:\.scope)?$`)
	lxcPattern         = regexp.MustCompile(`(?m)/lxc/[^/]+$`)
	crioPattern        = regexp.MustCompile(`(?m)/crio-[0-9a-f]{12,64}\.scope$`)
)

func parseCgroupForContainer() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	lower := strings.ToLower(string(data))

	// Order: specific runtime before generic container.
	if dockerIDPattern.FindStringIndex(lower) != nil || dockerScopePattern.FindStringIndex(lower) != nil {
		return "docker"
	}
	if podmanPattern.FindStringIndex(lower) != nil {
		return "podman"
	}
	if crioPattern.FindStringIndex(lower) != nil {
		return "container" // CRI-O generic
	}
	if kubePattern.FindStringIndex(lower) != nil {
		return "kubernetes"
	}
	if lxcPattern.FindStringIndex(lower) != nil {
		return "lxc"
	}

	return ""
}
