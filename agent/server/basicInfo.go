package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	monitoring "github.com/lyhbdw/komari-lite/agent/monitoring/unit"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
	"github.com/lyhbdw/komari-lite/agent/version"

	pkg_flags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
)

var flags = pkg_flags.GlobalConfig

// Reconnects refresh aged addresses, not every fresh success/negative cache entry.
func onReconnected() {
	monitoring.RefreshIPCacheOnReconnect()
	go UpdateBasicInfo()
}

func DoUploadBasicInfoWorks() {
	ticker := time.NewTicker(time.Duration(flags.InfoReportInterval) * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		err := uploadBasicInfo()
		if err != nil {
			log.Println("Error uploading basic info:", err)
		}
	}
}
func UpdateBasicInfo() {
	err := uploadBasicInfo()
	if err != nil {
		log.Println("Error uploading basic info:", err)
	} else {
		log.Println("Basic info uploaded successfully")
	}
}

var staticBasicInfo struct {
	sync.Once
	data map[string]interface{}
}
var basicInfoUpload struct {
	sync.Mutex
	lastPayload string
	sentAt      time.Time
}

func cachedBasicInfo() map[string]interface{} {
	staticBasicInfo.Do(func() { staticBasicInfo.data = collectStaticBasicInfo() })
	data := make(map[string]interface{}, len(staticBasicInfo.data)+2)
	for k, v := range staticBasicInfo.data {
		data[k] = v
	}
	return data
}

func collectStaticBasicInfo() map[string]interface{} {
	cpu := monitoring.CpuStaticInfo()
	return map[string]interface{}{
		"cpu_name":           cpu.CPUName,
		"cpu_cores":          cpu.CPUCores,
		"cpu_physical_cores": cpu.CPUPhysicalCores,
		"arch":               cpu.CPUArchitecture,
		"os":                 monitoring.OSName(),
		"kernel_version":     monitoring.KernelVersion(),
		"mem_total":          monitoring.Ram().Total,
		"swap_total":         monitoring.Swap().Total,
		"disk_total":         monitoring.Disk().Total,
		"gpu_name":           monitoring.GpuName(),
		"virtualization":     monitoring.Virtualized(),
		"version":            version.Current,
	}

}

func uploadBasicInfo() error {
	basicInfoUpload.Lock()
	defer basicInfoUpload.Unlock()
	data := cachedBasicInfo()
	ipv4, ipv6, _ := monitoring.GetIPAddress()
	data["ipv4"], data["ipv6"] = ipv4, ipv6
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	key := flags.Endpoint + "\n" + flags.Token + "\n" + string(payload)
	if key == basicInfoUpload.lastPayload && time.Since(basicInfoUpload.sentAt) < time.Duration(flags.InfoReportInterval)*time.Minute {
		return nil
	}
	if err = tryUploadDataWithProtocol(data); err != nil {
		return err
	}
	basicInfoUpload.lastPayload, basicInfoUpload.sentAt = key, time.Now()
	return nil
}

func tryUploadDataWithProtocol(data map[string]interface{}) error {
	payload := v2.BuildBasicInfoPayload(data)
	_, err := postV2Payload(context.Background(), payload, 30*time.Second, false)
	if err != nil {
		return err
	}
	markMigrationReady()
	return nil
}

func markMigrationReady() {
	path := strings.TrimSpace(flags.MigrationReadyFile)
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return
		}
		log.Printf("Unable to write migration readiness marker: %v", err)
		return
	}
	if _, err := file.WriteString("ready\n"); err != nil {
		log.Printf("Unable to write migration readiness marker: %v", err)
	}
	if err := file.Close(); err != nil {
		log.Printf("Unable to close migration readiness marker: %v", err)
	}
}
