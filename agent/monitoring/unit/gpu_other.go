//go:build !linux

package monitoring

// GpuName returns GPU device names for non-Linux platforms
func readGPUName() string {
	return "None"
}
