package server

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Tumb1er1376/komari-agent-lite/dnsresolver"
	monitoring "github.com/Tumb1er1376/komari-agent-lite/monitoring/unit"
	"github.com/Tumb1er1376/komari-agent-lite/protocol/transport"
	v2 "github.com/Tumb1er1376/komari-agent-lite/protocol/v2"
	"github.com/Tumb1er1376/komari-agent-lite/version"

	pkg_flags "github.com/Tumb1er1376/komari-agent-lite/cmd/flags"
)

var flags = pkg_flags.GlobalConfig

func DoUploadBasicInfoWorks() {
	ticker := time.NewTicker(time.Duration(flags.InfoReportInterval) * time.Minute)
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
func uploadBasicInfo() error {
	cpu := monitoring.CpuStaticInfo()

	osname := monitoring.OSName()
	kernelVersion := monitoring.KernelVersion()
	ipv4, ipv6, _ := monitoring.GetIPAddress()

	data := map[string]interface{}{
		"cpu_name":           cpu.CPUName,
		"cpu_cores":          cpu.CPUCores,
		"cpu_physical_cores": cpu.CPUPhysicalCores,
		"arch":               cpu.CPUArchitecture,
		"os":                 osname,
		"kernel_version":     kernelVersion,
		"ipv4":               ipv4,
		"ipv6":               ipv6,
		"mem_total":          monitoring.Ram().Total,
		"swap_total":         monitoring.Swap().Total,
		"disk_total":         monitoring.Disk().Total,
		"gpu_name":           monitoring.GpuName(),
		"virtualization":     monitoring.Virtualized(),
		"version":            version.Current,
	}

	return tryUploadData(data)
}

func tryUploadData(data map[string]interface{}) error {
	return tryUploadDataWithProtocol(data)
}

func tryUploadDataWithProtocol(data map[string]interface{}) error {
	endpoint := strings.TrimSuffix(flags.Endpoint, "/") + "/api/clients/v2/rpc"
	payload := v2.BuildBasicInfoPayload(data)
	body := payload
	compressed := false
	if !flags.DisableCompression {
		if gz, err := transport.GzipBytes(payload); err == nil {
			body = gz
			compressed = true
		}
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	addAgentAuthorization(req)
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := dnsresolver.GetHTTPClientWithPreference(30*time.Second, flags.PreferIPVersion)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := readBoundedBody(resp.Body)
	if err != nil {
		return err
	}
	message := string(respBody)

	if resp.StatusCode != http.StatusOK {
		return &httpStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: message}
	}
	if len(bytes.TrimSpace(respBody)) > 0 {
		if _, err := parseV2Response(respBody); err != nil {
			return err
		}
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
