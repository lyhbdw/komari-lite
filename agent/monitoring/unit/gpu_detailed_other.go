//go:build !linux

package monitoring

// GetDetailedGPUHost 在非 Linux 平台降级返回空
func GetDetailedGPUHost() ([]string, error) {
	return nil, nil
}

// GetDetailedGPUInfo 在非 Linux 平台降级返回空
func GetDetailedGPUInfo() ([]DetailedGPUInfo, error) {
	return nil, nil
}
