//go:build !linux

package monitoring

func getSystemBootID() string {
	return "generic-boot-id"
}

func isCountedElsewhere(nicName string) bool {
	return false
}
