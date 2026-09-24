package notifier

import (
	"fmt"
	"time"

	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/models"
	messageevent "github.com/komari-monitor/komari/database/models/messageEvent"
	"github.com/komari-monitor/komari/internal/config"
	logger "github.com/komari-monitor/komari/utils/log"
	"github.com/komari-monitor/komari/utils/messageSender"
)

// expireCache 记录每个客户端最近一次发送到期提醒时所处的"剩余天数档位"，
// 避免同一天内反复提醒。key: "expire:"+clientUUID, value: int 剩余天数档位
var expireCache = expireStateMap{}

const (
	// expireReminderDaysDefault 默认提前提醒天数（可由 expire_notification_lead_days 配置）
	expireReminderDaysDefault = 7
	// expireLongTermYears 超过该年限视为长期/一次性账单，不提醒
	expireLongTermYears = 100
)

// CheckExpire 检查各客户端的到期时间，在到期前 N 天（每天一次）与已过期时发送提醒。
// 由调度器每小时调用一次（按剩余天数档位去重，同一天不会重复提醒）。
func CheckExpire() {
	enabled, err := config.GetAs[bool](config.ExpireNotificationEnabledKey, false)
	if err != nil || !enabled {
		return
	}
	leadDays, err := config.GetAs[int](config.ExpireNotificationLeadDaysKey, expireReminderDaysDefault)
	if err != nil || leadDays < 0 {
		leadDays = expireReminderDaysDefault
	}

	allClients, err := clients.GetAllClientBasicInfo()
	if err != nil {
		logger.Errorf("notifier", "expire check: failed to list clients: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, c := range allClients {
		if c.ExpiredAt == nil {
			continue
		}
		expireTime := c.ExpiredAt.UTC()
		// 未设置到期时间（0002 年附近）或长期账单，跳过
		if expireTime.Year() < 2 || expireTime.After(now.AddDate(expireLongTermYears, 0, 0)) {
			continue
		}

		daysLeft := int(expireTime.Sub(now).Hours() / 24)

		var emoji, msg string
		switch {
		case daysLeft < 0:
			emoji = "⛔"
			msg = fmt.Sprintf("expired %d day(s) ago", -daysLeft)
		case daysLeft <= leadDays:
			emoji = "⏰"
			msg = fmt.Sprintf("expiring in %d day(s)", daysLeft)
		default:
			// 还早，不提醒；同时清掉旧档位，续费后重新进入提醒窗口时能再次触发
			expireCache.take(c.UUID)
			continue
		}

		// 同一档位（同一天）只提醒一次
		if !expireCache.markNotified(c.UUID, daysLeft) {
			continue
		}

		_ = messageSender.SendNotification(models.EventMessage{
			Event:   messageevent.Expire,
			Clients: []models.Client{c},
			Time:    time.Now().UTC(),
			Emoji:   emoji,
			Message: fmt.Sprintf("• %s: %s (expires %s)", c.Name, msg, expireTime.Format("2006-01-02")),
		})
	}
}
