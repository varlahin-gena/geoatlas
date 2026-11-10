package collector

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var commToTarget = map[string]string{
	"geoatlas": "backend",
	"clickhouse-serv": "clickhouse",
	"clickhouse":      "clickhouse",
	"syslog-ng":       "syslog-ng",
	"syslog-ng-main":  "syslog-ng",
	"nginx":           "frontend",
	"stats-collecto":  "stats-collector",
	"stats-collector": "stats-collector",
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// extractCgroupBasename выделяет последний компонент cgroup-пути,
// например "docker-XXXX.scope". Работает и с относительными путями
// (cgroup namespaces, "0::/../docker-XXX.scope"), и с абсолютными.
func extractCgroupBasename(content string) string {
	for _, line := range strings.Split(content, "\n") {
		var path string
		if strings.HasPrefix(line, "0::") { // cgroups v2
			path = strings.TrimPrefix(line, "0::")
		} else { // cgroups v1
			parts := strings.SplitN(line, ":", 3)
			if len(parts) == 3 {
				path = parts[2]
			}
		}
		if path == "" {
			continue
		}
		segments := strings.Split(path, "/")
		last := segments[len(segments)-1]
		if last != "" && last != "." && last != ".." {
			return last
		}
	}
	return ""
}

func (c *Collector) resolveCgroup(basename string) string {
	if cached, ok := c.cgroupCache[basename]; ok {
		return cached
	}

	candidates := []string{
		filepath.Join(c.cfg.CgroupRoot, "system.slice", basename),
		filepath.Join(c.cfg.CgroupRoot, basename),
	}
	if strings.HasPrefix(basename, "docker-") && strings.HasSuffix(basename, ".scope") {
		id := strings.TrimSuffix(strings.TrimPrefix(basename, "docker-"), ".scope")
		candidates = append(candidates, filepath.Join(c.cfg.CgroupRoot, "docker", id))
	}
	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			c.cgroupCache[basename] = cand
			return cand
		}
	}

	// Рекурсивный fallback (дорогой, но результат кэшируется).
	var found string
	_ = filepath.Walk(c.cfg.CgroupRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if info != nil && info.IsDir() && info.Name() == basename {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if found != "" {
		c.cgroupCache[basename] = found
	}
	return found
}

func (c *Collector) FindCgroupPaths() map[string]string {
	result := map[string]string{}

	entries, err := os.ReadDir(c.cfg.HostProcRoot)
	if err != nil {
		log.Printf("cannot read %s: %v", c.cfg.HostProcRoot, err)
		return result
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}

		comm := readFile(filepath.Join(c.cfg.HostProcRoot, e.Name(), "comm"))
		if comm == "" {
			continue
		}
		target, ok := commToTarget[comm]
		if !ok {
			continue
		}
		if _, exists := result[target]; exists {
			continue
		}

		cg := readFile(filepath.Join(c.cfg.HostProcRoot, e.Name(), "cgroup"))
		if cg == "" {
			continue
		}
		basename := extractCgroupBasename(cg)
		if basename == "" {
			continue
		}
		if fullPath := c.resolveCgroup(basename); fullPath != "" {
			result[target] = fullPath
		}
	}
	return result
}

// readCPUUsage возвращает совокупное CPU-время в наносекундах.
func readCPUUsage(cgroupPath string) (float64, bool) {
	// cgroups v2: cpu.stat -> usage_usec (микросекунды).
	if stat := readFile(filepath.Join(cgroupPath, "cpu.stat")); stat != "" {
		for _, line := range strings.Split(stat, "\n") {
			if strings.HasPrefix(line, "usage_usec") {
				parts := strings.Fields(line)
				if len(parts) == 2 {
					if v, err := strconv.ParseFloat(parts[1], 64); err == nil {
						return v * 1000.0, true // usec -> ns
					}
				}
			}
		}
	}
	// cgroups v1: cpuacct.usage (наносекунды).
	if v := readFile(filepath.Join(cgroupPath, "cpuacct.usage")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// parseMemoryStatKey читает одно числовое поле из memory.stat (cgroup v1/v2).
func parseMemoryStatKey(statContent, key string) (float64, bool) {
	prefix := key + " "
	for _, line := range strings.Split(statContent, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, prefix)), 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	return 0, false
}

// memUsageNoCache — формула Docker/Dozzle: usage минус reclaimable page cache
// (inactive_file / total_inactive_file). Без этого ClickHouse и др. выглядят
// в разы «тяжелее» из‑за file cache в memory.current.
func memUsageNoCache(usage float64, statContent string) float64 {
	if statContent == "" {
		return usage
	}
	// cgroup v1: total_inactive_file; v2 / Docker: inactive_file
	for _, key := range []string{"total_inactive_file", "inactive_file"} {
		if inactive, ok := parseMemoryStatKey(statContent, key); ok && inactive < usage {
			return usage - inactive
		}
	}
	return usage
}

func readMemoryBytes(cgroupPath string) (float64, bool) {
	stat := readFile(filepath.Join(cgroupPath, "memory.stat"))
	if v := readFile(filepath.Join(cgroupPath, "memory.current")); v != "" { // v2
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return memUsageNoCache(f, stat), true
		}
	}
	if v := readFile(filepath.Join(cgroupPath, "memory.usage_in_bytes")); v != "" { // v1
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return memUsageNoCache(f, stat), true
		}
	}
	return 0, false
}

// detectHostCPUCount читает число ядер хоста из HOST_PROC/cpuinfo.
// Fallback: runtime.NumCPU(); минимум 1.
func detectHostCPUCount(hostProcRoot string) int {
	path := filepath.Join(hostProcRoot, "cpuinfo")
	content := readFile(path)
	if content != "" {
		n := 0
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "processor") {
				n++
			}
		}
		if n > 0 {
			return n
		}
	}
	if n := runtime.NumCPU(); n > 0 {
		return n
	}
	return 1
}

// cpuPercent считает загрузку с прошлого замера.
// 100% == все ядра хоста (шкала как в Doozle); обычно ≤100% на один контейнер.
func (c *Collector) cpuPercent(name string, usageNs float64, now time.Time) (float64, bool) {
	prev, ok := c.cpuPrev[name]
	c.cpuPrev[name] = cpuDelta{usage: usageNs, at: now}
	if !ok || prev.at.IsZero() {
		return 0, false // первый замер — процент неопределён
	}
	dt := now.Sub(prev.at).Seconds()
	if dt <= 0 {
		return 0, false
	}
	du := (usageNs - prev.usage) / 1e9 // ns -> сек CPU
	if du < 0 {
		return 0, false // счётчик сбросился (рестарт контейнера)
	}
	nCPU := c.hostCPUCount
	if nCPU < 1 {
		nCPU = 1
	}
	return (du / dt) / float64(nCPU) * 100.0, true
}

func (c *Collector) collectContainerMetrics(ts time.Time) []Metric {
	out := []Metric{}
	paths := c.FindCgroupPaths()
	if len(paths) == 0 {
		log.Printf("warning: no container cgroups found in %s", c.cfg.HostProcRoot)
		return out
	}

	for target, cgPath := range paths {
		if mem, ok := readMemoryBytes(cgPath); ok {
			out = append(out, Metric{Timestamp: ts, Type: "container", Target: target, Name: "mem_bytes", Value: mem})
		}
		if usage, ok := readCPUUsage(cgPath); ok {
			if pct, ok := c.cpuPercent(target, usage, ts); ok {
				out = append(out, Metric{Timestamp: ts, Type: "container", Target: target, Name: "cpu_pct", Value: pct})
			}
		}
	}
	return out
}
