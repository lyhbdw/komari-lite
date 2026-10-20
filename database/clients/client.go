package clients

import (
	"context"
	"encoding/json"
	"fmt"
	logger "github.com/lyhbdw/komari-lite/utils/log"
	"math"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/database/tasks"
	"github.com/lyhbdw/komari-lite/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// A snapshot is immutable after publication; indexes keep token/UUID hits O(1).
type clientMetadata struct {
	clients []models.Client
	byUUID  map[string]int
	byToken map[string]int
}

var (
	clientsCacheMu         sync.RWMutex
	cachedClients          *clientMetadata
	clientsCacheDB         *gorm.DB
	clientsCachedAt        time.Time
	clientsCacheTTL        = 3 * time.Second
	clientsCacheGeneration uint64
)

func InvalidateClientsCache() {
	clientsCacheMu.Lock()
	clientsCacheGeneration++
	cachedClients = nil
	clientsCacheDB = nil
	clientsCacheMu.Unlock()
}

func cloneClient(client models.Client) models.Client {
	if client.ExpiredAt != nil {
		expires := *client.ExpiredAt
		client.ExpiredAt = &expires
	}
	return client
}

func cloneClients(src []models.Client) []models.Client {
	result := make([]models.Client, len(src))
	for i, client := range src {
		result[i] = cloneClient(client)
	}
	return result
}

func DeleteClient(clientUuid string) error {
	defer InvalidateClientsCache()
	db := dbcore.GetDBInstance()
	err := db.Delete(&models.Client{}, "uuid = ?", clientUuid).Error
	if err != nil {
		return err
	}
	return nil
}

func SaveClientInfo(update map[string]interface{}) error {
	db := dbcore.GetDBInstance()
	clientUUID, ok := update["uuid"].(string)
	if !ok || clientUUID == "" {
		return fmt.Errorf("invalid client UUID")
	}

	// 确保更新的字段不为空
	if len(update) == 0 {
		return fmt.Errorf("no fields to update")
	}

	update["updated_at"] = time.Now().UTC()

	toFloat64 := func(value interface{}) (float64, bool) {
		switch typed := value.(type) {
		case float64:
			return typed, true
		case float32:
			return float64(typed), true
		case int:
			return float64(typed), true
		case int8:
			return float64(typed), true
		case int16:
			return float64(typed), true
		case int32:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case uint:
			return float64(typed), true
		case uint8:
			return float64(typed), true
		case uint16:
			return float64(typed), true
		case uint32:
			return float64(typed), true
		case uint64:
			return float64(typed), true
		case json.Number:
			parsed, err := typed.Float64()
			if err != nil {
				return 0, false
			}
			return parsed, true
		default:
			return 0, false
		}
	}

	checkOptionalInt := func(name, key string, maxValue float64) error {
		value, exists := update[key]
		if !exists || value == nil {
			return nil
		}

		numericValue, ok := toFloat64(value)
		if !ok {
			return fmt.Errorf("%s must be a valid number", name)
		}
		if numericValue < 0 || numericValue > maxValue {
			return fmt.Errorf("%s must be a valid non-negative number: %v", name, value)
		}
		return nil
	}

	verify := func(update map[string]interface{}) error {
		if err := checkOptionalInt("Cpu.Cores", "cpu_cores", math.MaxInt-1); err != nil {
			return err
		}
		if err := checkOptionalInt("Cpu.PhysicalCores", "cpu_physical_cores", math.MaxInt-1); err != nil {
			return err
		}
		if err := checkOptionalInt("Ram.Total", "mem_total", math.MaxInt64-1); err != nil {
			return err
		}
		if err := checkOptionalInt("Swap.Total", "swap_total", math.MaxInt64-1); err != nil {
			return err
		}
		if err := checkOptionalInt("Disk.Total", "disk_total", math.MaxInt64-1); err != nil {
			return err
		}
		return nil
	}

	if err := verify(update); err != nil {
		return err
	}

	defer InvalidateClientsCache()
	err := db.Model(&models.Client{}).Where("uuid = ?", clientUUID).Updates(update).Error
	if err != nil {
		return err
	}
	return nil
}

// CreateClient 创建新客户端
func CreateClient() (clientUUID, token string, err error) {
	return createClient("")
}

func createClient(name string) (clientUUID, token string, err error) {
	db := dbcore.GetDBInstance()
	token = utils.GenerateToken()
	clientUUID = uuid.New().String()
	if name == "" {
		name = "client_" + clientUUID[0:8]
	}

	client := models.Client{
		UUID:      clientUUID,
		Token:     token,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	defer InvalidateClientsCache()
	err = db.Create(&client).Error
	if err != nil {
		return "", "", err
	}
	if err := tasks.AddDefaultOnClientUUID(clientUUID); err != nil {
		logger.ErrorArgs("clients", "Failed to apply default-on ping tasks to new client:", err)
	}
	return clientUUID, token, nil
}

func CreateClientWithName(name string) (clientUUID, token string, err error) {
	return createClient(name)
}

func GetClientByUUID(uuid string) (client models.Client, err error) {
	metadata, err := getClientMetadata(context.Background())
	if err != nil {
		return models.Client{}, err
	}
	i, ok := metadata.byUUID[uuid]
	if !ok {
		return models.Client{}, gorm.ErrRecordNotFound
	}
	return cloneClient(metadata.clients[i]), nil
}

func GetClientTokenByUUID(uuid string) (token string, err error) {
	client, err := GetClientByUUID(uuid)
	if err != nil {
		return "", err
	}
	return client.Token, nil
}

func GetAllClientBasicInfo() (clients []models.Client, err error) {
	return GetAllClientBasicInfoWithContext(context.Background())
}

// GetAllClientBasicInfoWithContext preserves caller cancellation/deadlines for
// cache misses while serving cache hits without database work.
func GetAllClientBasicInfoWithContext(ctx context.Context) (clients []models.Client, err error) {
	metadata, err := getClientMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return cloneClients(metadata.clients), nil
}

func getClientMetadata(ctx context.Context) (*clientMetadata, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rawDB := dbcore.GetDBInstance()
	clientsCacheMu.RLock()
	if cachedClients != nil && clientsCacheDB == rawDB && time.Since(clientsCachedAt) < clientsCacheTTL {
		metadata := cachedClients
		clientsCacheMu.RUnlock()
		return metadata, nil
	}
	generation := clientsCacheGeneration
	clientsCacheMu.RUnlock()

	var clients []models.Client
	if err := rawDB.WithContext(ctx).Find(&clients).Error; err != nil {
		return nil, err
	}

	metadata := &clientMetadata{
		clients: clients,
		byUUID:  make(map[string]int, len(clients)),
		byToken: make(map[string]int, len(clients)),
	}
	for i, client := range clients {
		metadata.byUUID[client.UUID] = i
		metadata.byToken[client.Token] = i
	}
	clientsCacheMu.Lock()
	// Do not republish a pre-write snapshot after invalidation.
	if generation == clientsCacheGeneration {
		cachedClients = metadata
		clientsCacheDB = rawDB
		clientsCachedAt = time.Now()
	}
	clientsCacheMu.Unlock()

	return metadata, nil
}

func SaveClient(updates map[string]interface{}) error {
	db := dbcore.GetDBInstance()
	clientUUID, ok := updates["uuid"].(string)
	if !ok || clientUUID == "" {
		return fmt.Errorf("invalid client UUID")
	}

	// 确保更新的字段不为空
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	if v, exists := updates["traffic_limit"]; exists {
		if val, ok := v.(float64); ok {
			if val < 0 || val > math.MaxInt64-1 {
				return fmt.Errorf("traffic_limit must be a valid non-negative int64 value, got %v", val)
			}
		}
	}
	if value, exists := updates["expired_at"]; exists {
		switch typed := value.(type) {
		case nil:
			updates["expired_at"] = nil
		case time.Time:
			updates["expired_at"] = typed.UTC()
		case *time.Time:
			if typed == nil {
				updates["expired_at"] = nil
			} else {
				updates["expired_at"] = typed.UTC()
			}
		case string:
			stamp, err := time.Parse(time.RFC3339Nano, typed)
			if err != nil {
				return fmt.Errorf("expired_at must be an RFC3339 timestamp with a timezone: %w", err)
			}
			updates["expired_at"] = stamp.UTC()
		default:
			return fmt.Errorf("expired_at must be an RFC3339 timestamp with a timezone")
		}
	}

	updates["updated_at"] = time.Now().UTC()

	defer InvalidateClientsCache()
	err := db.Model(&models.Client{}).Where("uuid = ?", clientUUID).Updates(updates).Error
	if err != nil {
		return err
	}
	return nil
}
