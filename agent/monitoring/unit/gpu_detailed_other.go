//go:build !linux

package monitoring

// DetailedGPUInfo 详细GPU信息结构体
type DetailedGPUInfo struct {
	Name        string  `json:"name"`         // GPU型号
	MemoryTotal uint64  `json:"memory_total"` // 总显存 (字节)
	MemoryUsed  uint64  `json:"memory_used"`  // 已用显存 (字节)
	Utilization float64 `json:"utilization"`  // GPU使用率 (0-100)
	Temperature uint64  `json:"temperature"`  // 温度 (摄氏度)
}

// GetDetailedGPUHost 在非 Linux 平台降级返回空
func GetDetailedGPUHost() ([]string, error) {
	return nil, nil
}

// GetDetailedGPUInfo 在非 Linux 平台降级返回空
func GetDetailedGPUInfo() ([]DetailedGPUInfo, error) {
	return nil, nil
}
