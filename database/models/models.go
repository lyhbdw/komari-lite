package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Client represents a registered client device
type Client struct {
	UUID             string     `json:"uuid,omitempty" gorm:"type:varchar(36);primaryKey"`
	Token            string     `json:"token,omitempty" gorm:"type:varchar(255);unique;not null"`
	Name             string     `json:"name" gorm:"type:varchar(100)"`
	CpuName          string     `json:"cpu_name" gorm:"type:varchar(100)"`
	Virtualization   string     `json:"virtualization" gorm:"type:varchar(50)"`
	Arch             string     `json:"arch" gorm:"type:varchar(50)"`
	CpuCores         int        `json:"cpu_cores" gorm:"type:int"`
	CpuPhysicalCores int        `json:"cpu_physical_cores" gorm:"type:int"`
	OS               string     `json:"os" gorm:"type:varchar(100)"`
	KernelVersion    string     `json:"kernel_version" gorm:"type:varchar(100)"`
	GpuName          string     `json:"gpu_name" gorm:"type:varchar(100)"`
	IPv4             string     `json:"ipv4,omitempty" gorm:"type:varchar(100)"`
	IPv6             string     `json:"ipv6,omitempty" gorm:"type:varchar(100)"`
	Region           string     `json:"region" gorm:"type:varchar(100)"`
	Remark           string     `json:"remark,omitempty" gorm:"type:longtext"`
	PublicRemark     string     `json:"public_remark,omitempty" gorm:"type:longtext"`
	MemTotal         int64      `json:"mem_total" gorm:"type:bigint"`
	SwapTotal        int64      `json:"swap_total" gorm:"type:bigint"`
	DiskTotal        int64      `json:"disk_total" gorm:"type:bigint"`
	Version          string     `json:"version,omitempty" gorm:"type:varchar(100)"`
	Weight           int        `json:"weight" gorm:"type:int"`
	Price            float64    `json:"price"`
	Premium          float64    `json:"premium"` // 购入溢价/附加费用，计入价值统计
	PremiumCurrency  string     `json:"premium_currency" gorm:"type:varchar(20);default:'¥'"`
	BillingCycle     int        `json:"billing_cycle"`
	AutoRenewal      bool       `json:"auto_renewal" gorm:"default:false"` // 是否自动续费
	Currency         string     `json:"currency" gorm:"type:varchar(20);default:'$'"`
	ExpiredAt        *time.Time `json:"expired_at" gorm:"type:timestamp"`
	Group            string     `json:"group" gorm:"type:varchar(100)"`
	Tags             string     `json:"tags" gorm:"type:text"` // split by ';'
	Hidden           bool       `json:"hidden" gorm:"default:false"`
	TrafficLimit     int64      `json:"traffic_limit" gorm:"type:bigint"`
	TrafficLimitType string     `json:"traffic_limit_type" gorm:"type:varchar(10);default:'max'"` // 流量阈值类型：sum max min up down
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// User represents an authenticated user
type User struct {
	UUID      string    `json:"uuid,omitempty" gorm:"type:varchar(36);primaryKey"`
	Username  string    `json:"username" gorm:"type:varchar(50);unique;not null"`
	Passwd    string    `json:"passwd,omitempty" gorm:"type:varchar(255);not null"` // Hashed password
	// NOTE(Lite): SSOType/SSOID 仅为数据库兼容保留（便于从原版迁移数据），
	// Lite 版已关闭 OAuth/OIDC 登录，业务代码中不再使用这两个字段，请勿删除。
	SSOType   string    `json:"sso_type" gorm:"type:varchar(20)"`                   // e.g., "github", "google"
	SSOID     string    `json:"sso_id" gorm:"type:varchar(100)"`                    // OAuth provider's user ID
	TwoFactor string    `json:"two_factor,omitempty" gorm:"type:varchar(255)"`      // 2FA secret
	Sessions  []Session `json:"sessions,omitempty" gorm:"foreignKey:UUID;references:UUID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Session manages user sessions
type Session struct {
	UUID            string    `json:"uuid" gorm:"type:varchar(36)"`
	Session         string    `json:"session" gorm:"type:varchar(255);primaryKey;uniqueIndex:idx_sessions_session;not null"`
	UserAgent       string    `json:"user_agent" gorm:"type:text"`
	Ip              string    `json:"ip" gorm:"type:varchar(100)"`
	LoginMethod     string    `json:"login_method" gorm:"type:varchar(50)"`
	LatestOnline    time.Time `json:"latest_online" gorm:"type:timestamp"`
	LatestUserAgent string    `json:"latest_user_agent" gorm:"type:text"`
	LatestIp        string    `json:"latest_ip" gorm:"type:varchar(100)"`
	Expires         time.Time `json:"expires" gorm:"not null"`
	CreatedAt       time.Time `json:"created_at"`
}

// Record logs client metrics over time
type Record struct {
	Client         string    `json:"client" gorm:"type:varchar(36);index"`
	Time           time.Time `json:"time" gorm:"index"`
	Cpu            float32   `json:"cpu" gorm:"type:decimal(5,2)"` // e.g., 75.50%
	Gpu            float32   `json:"gpu" gorm:"type:decimal(5,2)"`
	Ram            int64     `json:"ram" gorm:"type:bigint"`
	RamTotal       int64     `json:"ram_total" gorm:"type:bigint"`
	Swap           int64     `json:"swap" gorm:"type:bigint"`
	SwapTotal      int64     `json:"swap_total" gorm:"type:bigint"`
	Load           float32   `json:"load" gorm:"type:decimal(5,2)"`
	Temp           float32   `json:"temp" gorm:"type:decimal(5,2)"`
	Disk           int64     `json:"disk" gorm:"type:bigint"`
	DiskTotal      int64     `json:"disk_total" gorm:"type:bigint"`
	NetIn          int64     `json:"net_in" gorm:"type:bigint"`
	NetOut         int64     `json:"net_out" gorm:"type:bigint"`
	NetTotalUp     int64     `json:"net_total_up" gorm:"type:bigint"`
	NetTotalDown   int64     `json:"net_total_down" gorm:"type:bigint"`
	TrafficUp      int64     `json:"traffic_up" gorm:"type:bigint"`
	TrafficDown    int64     `json:"traffic_down" gorm:"type:bigint"`
	Process        int       `json:"process"`
	Connections    int       `json:"connections"`
	ConnectionsUdp int       `json:"connections_udp"`
	//Uptime         int64     `json:"uptime" gorm:"type:bigint"`
}

// GPURecord logs individual GPU metrics over time
type GPURecord struct {
	Client      string    `json:"client" gorm:"type:varchar(36);index"` // 客户端UUID
	Time        time.Time `json:"time" gorm:"index"`                    // 记录时间
	DeviceIndex int       `json:"device_index" gorm:"index"`            // GPU设备索引 (0,1,2...)
	DeviceName  string    `json:"device_name" gorm:"type:varchar(100)"` // GPU型号
	MemTotal    int64     `json:"mem_total" gorm:"type:bigint"`         // 显存总量(字节)
	MemUsed     int64     `json:"mem_used" gorm:"type:bigint"`          // 显存使用(字节)
	Utilization float32   `json:"utilization" gorm:"type:decimal(5,2)"` // GPU使用率(%)
	Temperature int       `json:"temperature"`                          // GPU温度(°C)
}

// StringArray represents a slice of strings stored as JSON in the database
// StringArray 存储为 JSON 的字符串切片类型
type StringArray []string

func (sa *StringArray) Scan(value interface{}) error {
	var bytes []byte
	switch v := value.(type) {
	case nil:
		*sa = StringArray{}
		return nil
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to scan StringArray: unsupported value type %T", value)
	}
	if len(bytes) == 0 {
		*sa = StringArray{}
		return nil
	}
	return json.Unmarshal(bytes, sa)
}

func (sa StringArray) Value() (driver.Value, error) {
	return json.Marshal(sa)
}

// Log records operational and system events
type Log struct {
	ID      uint      `json:"id,omitempty" gorm:"primaryKey;autoIncrement"`
	IP      string    `json:"ip" gorm:"type:varchar(45);"` // IPv4 or IPv6
	UUID    string    `json:"uuid" gorm:"type:varchar(36);"`
	Message string    `json:"message" gorm:"type:text;not null"`
	MsgType string    `json:"msg_type" gorm:"type:varchar(20);not null;index:idx_logs_msg_type_time,priority:1"`
	Time    time.Time `json:"time" gorm:"autoCreateTime;not null;index:idx_logs_time;index:idx_logs_msg_type_time,priority:2"`
}

// OfflineNotification 定义了离线通知规则：按客户端记录宽限期与上次通知时间。
type OfflineNotification struct {
	Client          string     `json:"client" gorm:"type:varchar(36);not null;index;unique;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:client;references:UUID"`
	ClientInfo      Client     `json:"client_info,omitempty" gorm:"foreignKey:Client;references:UUID"`
	Enable          bool       `json:"enable" gorm:"type:boolean;default:false"`
	Cooldown        int        `json:"cooldown" gorm:"type:int;not null;default:1800"`    // 冷却时间（秒），默认 30 分钟；0 表示不冷却
	GracePeriod     int        `json:"grace_period" gorm:"type:int;not null;default:180"` // 宽限期（秒），默认 3 分钟
	LastNotified    *time.Time `json:"last_notified"`                                     // 上次通知时间
	NotifiedOffline bool       `json:"notified_offline" gorm:"type:boolean;default:false"` // 是否处于已发送离线告警状态
}

// MessageSenderProvider 与 EventMessage 定义通知投递实体
type MessageSenderProvider struct {
	Name     string `json:"name" gorm:"primaryKey;unique;not null"`
	Addition string `json:"addition" gorm:"type:longtext" default:"{}"`
}

type EventMessage struct {
	Event   any       `json:"event"`
	Clients []Client  `json:"clients"`
	Time    time.Time `json:"time"`
	Message any       `json:"message"`
	Emoji   any       `json:"emoji"`
}

// Theme 相关元数据与配置模型
type Theme struct {
	Name          any           `json:"name"`
	Short         string        `json:"short"`
	Description   any           `json:"description"`
	Version       string        `json:"version"`
	Author        any           `json:"author"`
	URL           string        `json:"url"`
	Preview       string        `json:"preview"`
	Configuration Configuration `json:"configuration"`
}

type Configuration struct {
	Type string `json:"type"` // managed raw redirect
	Icon string `json:"icon"` // 图标
	Name any    `json:"name"`
	Data any    `json:"data"` // 配置数据
}

type ManagedThemeConfigurationItem struct {
	Key      string `json:"key"`
	Name     any    `json:"name"`
	Required bool   `json:"required"`
	Type     string `json:"type"` // string number select switch title textbox richtext nodes pingtasks
	Options  string `json:"options"`
	Default  any    `json:"default"`
	Help     any    `json:"help"`
}

type ThemeConfiguration struct {
	Short string `json:"short" gorm:"primaryKey;unique;not null"`
	Data  string `json:"data" gorm:"type:longtext" default:"{}"`
}
