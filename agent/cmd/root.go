package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/lyhbdw/komari-lite/agent/dnsresolver"
	"github.com/lyhbdw/komari-lite/agent/monitoring/netstatic"
	monitoring "github.com/lyhbdw/komari-lite/agent/monitoring/unit"
	"github.com/lyhbdw/komari-lite/agent/server"
	"github.com/lyhbdw/komari-lite/agent/version"

	pkg_flags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
)

var flags = pkg_flags.GlobalConfig

// fs 是 agent 唯一的命令行标志集合。Lite 版没有子命令，
// 所有配置都通过这里的 flag、环境变量或配置文件提供。
var fs = flag.NewFlagSet("komari-agent", flag.ContinueOnError)

// knownFlags 记录所有已注册的 flag 名（含短名），用于过滤未知 flag。
// 这复刻了之前 cobra 的 ParseErrorsWhitelist.UnknownFlags 行为：
// 未知 flag 会被静默忽略，而不是报错退出。
var knownFlags = map[string]struct{}{}

func regString(p *string, long, short, def, usage string) {
	fs.StringVar(p, long, def, usage)
	knownFlags[long] = struct{}{}
	if short != "" {
		fs.StringVar(p, short, def, usage)
		knownFlags[short] = struct{}{}
	}
}

func regBool(p *bool, long, short string, def bool, usage string) {
	fs.BoolVar(p, long, def, usage)
	knownFlags[long] = struct{}{}
	if short != "" {
		fs.BoolVar(p, short, def, usage)
		knownFlags[short] = struct{}{}
	}
}

func regInt(p *int, long, short string, def int, usage string) {
	fs.IntVar(p, long, def, usage)
	knownFlags[long] = struct{}{}
	if short != "" {
		fs.IntVar(p, short, def, usage)
		knownFlags[short] = struct{}{}
	}
}

func regFloat64(p *float64, long, short string, def float64, usage string) {
	fs.Float64Var(p, long, def, usage)
	knownFlags[long] = struct{}{}
	if short != "" {
		fs.Float64Var(p, short, def, usage)
		knownFlags[short] = struct{}{}
	}
}

// showHelp 为 --help 提供显式支持（标准库只内置了 -h）。
var showHelp bool

func init() {
	fs.SetOutput(os.Stderr)
	regBool(&showHelp, "help", "", false, "Show help message")
	regString(&flags.Token, "token", "t", "", "API token")
	regString(&flags.Endpoint, "endpoint", "e", "", "API endpoint")
	regFloat64(&flags.Interval, "interval", "i", 3.0, "Interval in seconds")
	regBool(&flags.IgnoreUnsafeCert, "ignore-unsafe-cert", "u", false, "Ignore unsafe certificate errors")
	regInt(&flags.MaxRetries, "max-retries", "r", 3, "Maximum number of retries")
	regInt(&flags.ReconnectInterval, "reconnect-interval", "c", 5, "Reconnect interval in seconds")
	regInt(&flags.InfoReportInterval, "info-report-interval", "", 5, "Interval in minutes for reporting basic info")
	regString(&flags.IncludeNics, "include-nics", "", "", "Comma-separated list of network interfaces to include")
	regString(&flags.ExcludeNics, "exclude-nics", "", "", "Comma-separated list of network interfaces to exclude")
	regString(&flags.IncludeMountpoints, "include-mountpoint", "", "", "Semicolon-separated list of mount points to include for disk statistics")
	regInt(&flags.MonthRotate, "month-rotate", "", 1, "Day of month to reset network statistics (0 to disable, default 1 = 1st of month)")
	regBool(&flags.MemoryIncludeCache, "memory-include-cache", "", false, "Include cache/buffer in memory usage")
	regBool(&flags.MemoryReportRawUsed, "memory-exclude-bcf", "", false, "Use \"raminfo.Used = v.Total - v.Free - v.Buffers - v.Cached\" calculation for memory usage")
	regString(&flags.CustomDNS, "custom-dns", "", "", "Custom DNS server to use (e.g. 8.8.8.8, 114.114.114.114). By default, the program uses the system DNS resolver.")
	regBool(&flags.EnableGPU, "gpu", "", false, "Enable detailed GPU monitoring (usage, memory, multi-GPU support)")
	regString(&flags.CustomIpv4, "custom-ipv4", "", "", "Custom IPv4 address to use")
	regString(&flags.CustomIpv6, "custom-ipv6", "", "", "Custom IPv6 address to use")
	regBool(&flags.GetIpAddrFromNic, "get-ip-addr-from-nic", "", false, "Get IP address from network interface")
	regString(&flags.ConfigFile, "config", "", "", "Path to the configuration file")
	regBool(&flags.DisableCompression, "disable-compression", "", false, "Disable v2 gzip/permessage-deflate compression")
	regString(&flags.PreferIPVersion, "prefer-ip-version", "", "", "Prefer IP version for dashboard connections: 4 or 6")
	regString(&flags.MigrationReadyFile, "migration-ready-file", "", "", "Write a one-time readiness marker after the first successful panel report")
}

// filterUnknownArgs 丢弃未注册的 flag（及其值），保留其余参数原样。
// "--" 之后的内容视为位置参数，不再解析。
func filterUnknownArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			out = append(out, args[i:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			}
			if _, ok := knownFlags[name]; ok {
				out = append(out, a)
				continue
			}
			// 未知 flag：若以空格分隔带值，一并丢弃该值。
			if !strings.Contains(a, "=") && i+1 < len(args) && len(args[i+1]) > 0 && args[i+1][0] != '-' {
				i++
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

// parseArgs 解析命令行参数。未知 flag 被忽略；位置参数不允许。
func parseArgs(args []string) error {
	if err := fs.Parse(filterUnknownArgs(args)); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unknown positional arguments: %v", fs.Args())
	}
	return nil
}

func run() error {
	loadFromEnv() // 从环境变量加载配置，覆盖解析
	if flags.ConfigFile != "" {
		bytes, err := os.ReadFile(flags.ConfigFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		err = json.Unmarshal(bytes, flags)
		if err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	}
	if flags.PreferIPVersion != "" && flags.PreferIPVersion != "4" && flags.PreferIPVersion != "6" {
		return fmt.Errorf("invalid --prefer-ip-version value %q: expected 4 or 6", flags.PreferIPVersion)
	}
	if err := flags.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	// 捕获中止信号，优雅退出
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown := newShutdownCoordinator(netstatic.Stop, os.Exit)
	go func() {
		<-stopCtx.Done()
		log.Printf("shutting down gracefully...")
		shutdown.shutdown(0)
	}()

	if flags.MonthRotate != 0 {
		err := netstatic.StartOrContinue()
		if err != nil {
			log.Println("Failed to start netstatic monitoring:", err)
		}
		nics, err := monitoring.InterfaceList()
		if err != nil {
			log.Println("Failed to get interface list for netstatic:", err)
		}
		err = netstatic.SetNewConfig(netstatic.NetStaticConfig{
			Nics: nics,
		})
		if err != nil {
			log.Println("Failed to set netstatic config:", err)
		}
	}

	log.Println("Komari Agent", version.Current)

	// 设置 DNS 解析行为
	if flags.CustomDNS != "" {
		dnsresolver.SetCustomDNSServer(flags.CustomDNS)
		log.Printf("Using custom DNS server: %s", flags.CustomDNS)
	} else {
		// 未设置则使用系统默认 DNS（不使用内置列表）
		log.Printf("Using system default DNS resolver")
	}

	diskList, err := monitoring.DiskList()
	if err != nil {
		log.Println("Failed to get disk list:", err)
	}
	log.Println("Monitoring Mountpoints:", diskList)
	interfaceList, err := monitoring.InterfaceList()
	if err != nil {
		log.Println("Failed to get interface list:", err)
	}
	log.Println("Monitoring Interfaces:", interfaceList)

	// 忽略不安全的证书
	if flags.IgnoreUnsafeCert {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	go server.DoUploadBasicInfoWorks()
	for {
		server.UpdateBasicInfo()
		server.EstablishWebSocketConnection()
	}
}

func Execute() {
	if err := parseArgs(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		log.Println(err)
		os.Exit(1)
	}
	if showHelp {
		fs.Usage()
		os.Exit(0)
	}
	if err := run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func loadFromEnv() {
	val := reflect.ValueOf(flags).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Get the env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Get the environment variable value
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		// Set the field based on its type
		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Bool:
			if strings.ToLower(envValue) == "true" || envValue == "1" {
				field.SetBool(true)
			}
		case reflect.Int:
			if intVal, err := strconv.Atoi(envValue); err == nil {
				field.SetInt(int64(intVal))
			}
		case reflect.Float64:
			if floatVal, err := strconv.ParseFloat(envValue, 64); err == nil {
				field.SetFloat(floatVal)
			}
		}
	}
}
