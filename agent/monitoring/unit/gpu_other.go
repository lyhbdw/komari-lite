//go:build !linux

package monitoring

// GpuName returns GPU device names for non-Linux platforms
func GpuName() string {
	return "None"
}
