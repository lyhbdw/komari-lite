package accounts

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Tumb1er1376/komari-monitor-lite/database/dbcore"
	"github.com/Tumb1er1376/komari-monitor-lite/database/models"
	messageevent "github.com/Tumb1er1376/komari-monitor-lite/database/models/messageEvent"
	"github.com/Tumb1er1376/komari-monitor-lite/internal/config"
	"github.com/Tumb1er1376/komari-monitor-lite/utils"
	"github.com/Tumb1er1376/komari-monitor-lite/utils/geoip"
	"github.com/Tumb1er1376/komari-monitor-lite/utils/messageSender"
)

func SessionIdentifier(session string) string {
	sum := sha256.Sum256([]byte(session))
	return fmt.Sprintf("%x", sum[:8])
}

func DeleteSessionByIdentifier(identifier string) error {
	db := dbcore.GetDBInstance()
	var sessions []models.Session
	if err := db.Find(&sessions).Error; err != nil {
		return err
	}
	for _, session := range sessions {
		if SessionIdentifier(session.Session) == identifier {
			return DeleteSession(session.Session)
		}
	}
	return nil
}

// GetAllSessions 获取所有会话
func GetAllSessions() (sessions []models.Session, err error) {
	db := dbcore.GetDBInstance()
	err = db.Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

// CreateSession 创建新会话
func CreateSession(uuid string, expires int, userAgent, ip, login_method string) (string, error) {
	db := dbcore.GetDBInstance()
	session := utils.GenerateRandomString(32)

	sessionRecord := models.Session{
		UUID:         uuid,
		Session:      session,
		Expires:      time.Now().UTC().Add(time.Duration(expires) * time.Second),
		UserAgent:    userAgent,
		Ip:           ip,
		LoginMethod:  login_method,
		LatestOnline: time.Now().UTC(),
	}
	go func() {
		LoginNotification, _ := config.GetAs[bool](config.LoginNotificationKey, false)
		if LoginNotification {
			ipAddr := net.ParseIP(ip)
			ipinfo, _ := geoip.GetGeoInfo(ipAddr)
			loc := "unknown"
			if ipinfo != nil && ipinfo.Name != "" {
				loc = ipinfo.Name
			}
			_ = messageSender.SendNotification(models.EventMessage{
				Event:   messageevent.Login,
				Time:    time.Now().UTC(),
				Message: fmt.Sprintf("%s: %s (%s)\n%s", login_method, ip, loc, userAgent),
				Emoji:   "🔑",
			})
		}
	}()

	err := db.Create(&sessionRecord).Error
	if err != nil {
		return "", err
	}
	return session, nil
}

// GetSession 根据会话 ID 获取 UUID
func GetSession(session string) (uuid string, err error) {
	db := dbcore.GetDBInstance()
	var sessionRecord models.Session
	err = db.Where("session = ?", session).First(&sessionRecord).Error
	if err != nil {
		return "", err
	}

	if time.Now().UTC().After(sessionRecord.Expires) {
		// 会话已过期，删除它
		_ = DeleteSession(session)
		return "", errors.New("session expired")
	}

	return sessionRecord.UUID, nil
}

func GetUserBySession(session string) (models.User, error) {
	db := dbcore.GetDBInstance()
	var sessionRecord models.Session
	err := db.Where("session = ?", session).First(&sessionRecord).Error
	if err != nil {
		return models.User{}, err
	}
	if time.Now().UTC().After(sessionRecord.Expires) {
		_ = DeleteSession(session)
		return models.User{}, errors.New("session expired")
	}
	return GetUserByUUID(sessionRecord.UUID)
}

// DeleteSession 删除指定会话
func DeleteSession(session string) (err error) {
	db := dbcore.GetDBInstance()
	result := db.Where("session = ?", session).Delete(&models.Session{})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func DeleteAllSessions() error {
	db := dbcore.GetDBInstance()
	result := db.Where("1 = 1").Delete(&models.Session{})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func UpdateLatest(session, useragent, ip string) error {
	db := dbcore.GetDBInstance()
	return db.Model(&models.Session{}).Where("session = ?", session).Updates(map[string]interface{}{
		"latest_online":     time.Now().UTC(),
		"latest_user_agent": useragent,
		"latest_ip":         ip,
	}).Error
}

func RemoveExpiredSessions() error {
	db := dbcore.GetDBInstance()
	result := db.Where("expires < ?", time.Now().UTC()).Delete(&models.Session{})
	if result.Error != nil {
		return result.Error
	}
	return nil
}
