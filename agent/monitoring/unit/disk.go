package monitoring

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

var (
	partitionsCacheMu   sync.RWMutex
	cachedPhysicalParts []disk.PartitionStat
	partitionsCachedAt  time.Time
	cachedDiskInfo      DiskInfo
	diskInfoCachedAt    time.Time
)

type DiskInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

func Disk() DiskInfo {
	partitionsCacheMu.RLock()
	if cachedDiskInfo.Total > 0 && time.Since(diskInfoCachedAt) < 15*time.Second {
		info := cachedDiskInfo
		partitionsCacheMu.RUnlock()
		return info
	}
	partitionsCacheMu.RUnlock()

	diskinfo := DiskInfo{}
	// 如果指定了自定义挂载点，只统计指定的挂载点
	if flags.IncludeMountpoints != "" {
		includeMounts := strings.Split(flags.IncludeMountpoints, ";")
		for _, mountpoint := range includeMounts {
			mountpoint = strings.TrimSpace(mountpoint)
			if mountpoint != "" {
				u, err := disk.Usage(mountpoint)
				if err != nil {
					continue
				} else {
					diskinfo.Total += u.Total
					diskinfo.Used += u.Used
				}
			}
		}
		partitionsCacheMu.Lock()
		cachedDiskInfo = diskinfo
		diskInfoCachedAt = time.Now()
		partitionsCacheMu.Unlock()
		return diskinfo
	}

	// 使用默认逻辑，排除临时文件系统和网络驱动器（缓存物理分区列表 15 秒）
	var physicalParts []disk.PartitionStat
			partitionsCacheMu.RLock()
			if cachedPhysicalParts != nil && time.Since(partitionsCachedAt) < 15*time.Second {
				physicalParts = cachedPhysicalParts
				partitionsCacheMu.RUnlock()
			} else {
				partitionsCacheMu.RUnlock()
				usage, err := disk.Partitions(true)
				if err == nil {
					for _, part := range usage {
						if isPhysicalDisk(part) {
							physicalParts = append(physicalParts, part)
						}
					}
					partitionsCacheMu.Lock()
					cachedPhysicalParts = physicalParts
					partitionsCachedAt = time.Now()
					partitionsCacheMu.Unlock()
				}
			}

			deviceMap := make(map[string]*disk.UsageStat)
			for _, part := range physicalParts {
				u, err := disk.Usage(part.Mountpoint)
				if err != nil {
					continue
				}

				deviceID := part.Device
				// ZFS去重: 基于 pool 名称 (例如 pool/dataset -> pool)
				if strings.ToLower(part.Fstype) == "zfs" {
					if idx := strings.Index(deviceID, "/"); idx != -1 {
						deviceID = deviceID[:idx]
					}
				}

				// 如果该设备已存在，且当前挂载点的 Total 更大，则替换（处理 quota 等情况）
				// 否则保留现有的（通常我们希望统计物理 pool 的总量）
				if existing, ok := deviceMap[deviceID]; ok {
					if u.Total > existing.Total {
						deviceMap[deviceID] = u
					}
				} else {
					deviceMap[deviceID] = u
				}
			}

			for _, u := range deviceMap {
				diskinfo.Total += u.Total
				diskinfo.Used += u.Used
			}
			partitionsCacheMu.Lock()
			cachedDiskInfo = diskinfo
			diskInfoCachedAt = time.Now()
			partitionsCacheMu.Unlock()
	return diskinfo
}

// isPhysicalDisk 判断分区是否为物理磁盘
func isPhysicalDisk(part disk.PartitionStat) bool {
	// 对于LXC等基于loop的根文件系统，始终包含根挂载点
	if part.Mountpoint == "/" {
		return true
	}
	mountpoint := strings.ToLower(part.Mountpoint)
	// 排除挂载点
	var mountpointsToExcludePerfix = []string{
		"/tmp",
		"/var/tmp",
		"/dev",
		"/run",
		"/var/lib/containers",
		"/var/lib/docker",
		"/proc",
		"/sys",
		"/sys/fs/cgroup",
		"/etc/resolv.conf",
		"/etc/host", // /etc/hosts,/etc/hostname
		"/nix/store",
	}
	for _, mp := range mountpointsToExcludePerfix {
		if mountpoint == mp || strings.HasPrefix(mountpoint, mp) {
			return false
		}
	}

	fstype := strings.ToLower(part.Fstype)

	// 针对 Linux autofs：排除自动挂载的 trigger，真实文件系统会作为单独分区出现不会被排除。
	// 将 autofs 视为“非物理磁盘”可以避免重复统计容量。
	if fstype == "autofs" && !strings.HasPrefix(part.Device, "/dev/") {
		return false
	}

	// 针对 Linux 下通过 ntfs-3g 挂载的 NTFS 分区 (fuseblk)，这是实际物理磁盘，不应排除
	if fstype == "fuseblk" {
		return true
	}

	// Android 的 /sdcard 通过 FUSE (/dev/fuse) 挂载真实存储，不是网络文件系统。
	// 限定挂载点，避免将其他 FUSE 文件系统计入磁盘统计。
	if part.Device == "/dev/fuse" && fstype == "fuse" && mountpoint == "/sdcard" {
		return true
	}

	var fstypeToExclude = []string{
		"tmpfs",
		"devtmpfs",
		"udev",
		"nfs",
		"cifs",
		"smb",
		"vboxsf",
		"9p",
		"fuse",
		"overlay",
		"proc",
		"devpts",
		"sysfs",
		"cgroup",
		"mqueue",
		"hugetlbfs",
		"debugfs",
		"binfmt_misc",
		"securityfs",
		"nullfs",
	}
	for _, fs := range fstypeToExclude {
		if fstype == fs || strings.HasPrefix(fstype, fs) {
			return false
		}
	}
	// Windows 网络驱动器通常是映射盘符，但不容易通过fstype判断
	// 可以通过opts判断，Windows网络驱动通常有相关选项
	for _, opt := range part.Opts {
		optLower := strings.ToLower(opt)
		if strings.Contains(optLower, "remote") || strings.Contains(optLower, "network") {
			return false
		}
	}

	// 虚拟内存
	if strings.HasPrefix(part.Device, "/dev/loop") {
		return false
	}

	return true
}

func DiskList() ([]string, error) {
	diskList := []string{}
	if flags.IncludeMountpoints != "" {
		includeMounts := strings.Split(flags.IncludeMountpoints, ";")
		for _, mountpoint := range includeMounts {
			mountpoint = strings.TrimSpace(mountpoint)
			if mountpoint != "" {
				diskList = append(diskList, mountpoint)
			}
		}
	} else {
		usage, err := disk.Partitions(true)
		if err != nil {
			return nil, err
		}

		// 同一物理设备只保留路径最短的根挂载点
		deviceMap := make(map[string]disk.PartitionStat)
		for _, part := range usage {
			if isPhysicalDisk(part) {
				deviceID := part.Device
				// ZFS去重: 基于 pool 名称
				if strings.ToLower(part.Fstype) == "zfs" {
					if idx := strings.Index(deviceID, "/"); idx != -1 {
						deviceID = deviceID[:idx]
					}
				}

				if existing, ok := deviceMap[deviceID]; ok {
					// 优先保留路径更短的挂载点 (e.g., /volume1 优于 /volume1/@appdata/...)
					if len(part.Mountpoint) < len(existing.Mountpoint) {
						deviceMap[deviceID] = part
					}
				} else {
					deviceMap[deviceID] = part
				}
			}
		}

		for _, part := range deviceMap {
			diskList = append(diskList, fmt.Sprintf("%s (%s)", part.Mountpoint, part.Fstype))
		}
	}
	return diskList, nil
}
