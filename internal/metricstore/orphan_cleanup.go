package metricstore

import (
	"context"
	"fmt"
)

// CleanupOrphanEntities 删除 metric store 中已不存在于 clients 表的实体
// （历史遗留：1.0.0 时代删除节点时没有清理指标数据，series/rollup 行残留）。
// 孤儿行不占多少空间，但会污染 EntityIDs 扫描，让已删除的节点重新出现在
// 历史查询的实体列表里。
//
// knownClientIDs 由调用方（server 层）从 clients 表读取后传入，避免
// metricstore → dbcore 的 import cycle（dbcore → migrations → metricstore）。
//
// 安全性说明：所有内置指标（含 ping.latency_ms / ping.loss）的 entity_id
// 都是 client UUID（ping 的 task 维度走 tags），因此与 clients 表比对是
// 安全的，不会误删仍有效的数据。
//
// 由每小时的 metrics:retention 调度任务调用；单个实体删除失败不中断整体
// 清理（下次调度重试），返回成功删除的数量。
func CleanupOrphanEntities(ctx context.Context, knownClientIDs map[string]struct{}) (int, error) {
	s := GetStore()
	if s == nil {
		return 0, fmt.Errorf("metric store not enabled")
	}

	entities, err := s.ListEntityIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list metric entities for orphan cleanup: %w", err)
	}

	deleted := 0
	for _, entityID := range entities {
		if _, ok := knownClientIDs[entityID]; ok {
			continue
		}
		if _, err := s.DeleteEntity(ctx, entityID); err != nil {
			// 记录但继续：单个实体失败不应中断整轮清理。
			continue
		}
		deleted++
	}
	return deleted, nil
}
