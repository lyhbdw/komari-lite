package models

import "time"

// OfflineNotification 定义了离线通知规则：按客户端记录宽限期与上次通知时间。
type OfflineNotification struct {
	Client       string     `json:"client" gorm:"type:varchar(36);not null;index;unique;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:client;references:UUID"`
	ClientInfo   Client     `json:"client_info,omitempty" gorm:"foreignKey:Client;references:UUID"`
	Enable       bool       `json:"enable" gorm:"type:boolean;default:false"`
	Cooldown     int        `json:"cooldown" gorm:"type:int;not null;default:1800"`    // 冷却时间（秒），默认 30 分钟；0 表示不冷却
	GracePeriod  int        `json:"grace_period" gorm:"type:int;not null;default:180"` // 宽限期（秒），默认 3 分钟
	LastNotified *time.Time `json:"last_notified"`                                     // 上次通知时间
}
