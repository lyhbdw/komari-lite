package monitoring

import "time"

const (
	gpuCommandTimeout     = 2 * time.Second
	gpuCommandOutputLimit = 1 << 20
)
