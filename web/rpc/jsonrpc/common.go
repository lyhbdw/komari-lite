package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/lyhbdw/komari-lite/database"
	"github.com/lyhbdw/komari-lite/database/clients"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/database/tasks"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"github.com/lyhbdw/komari-lite/pkg/metric"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
	"github.com/lyhbdw/komari-lite/utils"
	agent_runtime "github.com/lyhbdw/komari-lite/web/agent"

	"github.com/lyhbdw/komari-lite/utils/ttlcache"
	"golang.org/x/sync/singleflight"
)

// Ping summaries are sampled once per minute, not recomputed on every status
// tick. Task metadata participates in the key so edits revoke stale assignments.
var pingStatsCache = ttlcache.New(time.Minute)
var pingSummaryLoads singleflight.Group

type pingStat struct {
	Name        string  `json:"name"`
	Weight      int     `json:"weight"`
	Latest      int     `json:"latest"`
	Avg         int     `json:"avg"`
	Tail        float64 `json:"tail"`
	Loss        float64 `json:"loss"`
	Min         int     `json:"min"`
	Max         int     `json:"max"`
	Approximate bool    `json:"approximate,omitempty"`
}

func (s pingStat) MarshalJSON() ([]byte, error) {
	type plain pingStat
	var minimum *int
	if s.Min >= 0 {
		minimum = &s.Min
	}
	return json.Marshal(struct {
		plain
		Minimum *int `json:"min"`
	}{plain: plain(s), Minimum: minimum})
}

type cachedPingSummary struct {
	Fingerprint string
	Stats       map[string]pingStat
}

// getPingStatsForNode keeps the old internal helper for compatibility tests.
func getPingStatsForNode(uuid string, pingTasks []models.PingTask) map[string]pingStat {
	return getPingStatsForNodeContext(context.Background(), uuid, pingTasks)
}

func getPingStatsForNodeContext(ctx context.Context, uuid string, pingTasks []models.PingTask) map[string]pingStat {
	if ctx.Err() != nil {
		return map[string]pingStat{}
	}
	assigned := make(map[string]models.PingTask)
	for _, task := range pingTasks {
		if task.AppliesToClient(uuid) {
			// Membership was already checked. Fleet-wide client lists do not
			// affect this node's summary and must not inflate every cache key.
			task.Clients = nil
			assigned[strconv.FormatUint(uint64(task.Id), 10)] = task
		}
	}
	if uuid == "" || len(assigned) == 0 {
		return map[string]pingStat{}
	}
	fingerprint, _ := json.Marshal(assigned)
	cacheHit := func() (map[string]pingStat, bool) {
		if value, ok := pingStatsCache.Get(uuid); ok {
			cached := value.(cachedPingSummary)
			if cached.Fingerprint == string(fingerprint) {
				return cached.Stats, true
			}
		}
		return nil, false
	}
	if cached, ok := cacheHit(); ok {
		return cached
	}
	// Flight keys include the policy, while persistent cache keys are only UUIDs:
	// repeated task edits replace one entry rather than retaining every version.
	resultCh := pingSummaryLoads.DoChan(uuid+":"+string(fingerprint), func() (any, error) {
		if cached, ok := cacheHit(); ok {
			return cached, nil
		}
		queryCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stats, err := loadNodePingSummary(queryCtx, uuid, assigned)
		if err == nil {
			pingStatsCache.Set(uuid, cachedPingSummary{Fingerprint: string(fingerprint), Stats: stats})
		}
		return stats, err
	})
	select {
	case <-ctx.Done():
		return map[string]pingStat{}
	case result := <-resultCh:
		if result.Err != nil {
			return map[string]pingStat{}
		}
		return result.Val.(map[string]pingStat)
	}
}

func loadNodePingSummary(ctx context.Context, uuid string, assigned map[string]models.PingTask) (map[string]pingStat, error) {
	store := metricstore.GetStore()
	if store == nil {
		return nil, fmt.Errorf("metric store unavailable")
	}
	now := time.Now().UTC()
	start := now.Add(-time.Hour)
	interval := store.CompatibleSeriesInterval(start, now, time.Minute)
	loaded, err := store.SeriesBatch(ctx, metric.BatchSeriesQuery{
		Specs: []metric.BatchSeriesSpec{
			{MetricName: metricstore.MetricPingLatency, Aggregations: []metric.Aggregation{metric.AggAvg, metric.AggMin, metric.AggMax, metric.AggLast, metric.AggP50, metric.AggP99}, Interval: interval, PreserveSeries: true},
			{MetricName: metricstore.MetricPingLoss, Aggregations: []metric.Aggregation{metric.AggAvg}, Interval: interval, PreserveSeries: true},
		},
		EntityIDs: []string{uuid}, Start: start, End: now, Order: metric.OrderAsc,
	}, now)
	if err != nil {
		return nil, err
	}
	latency := loaded.Values[metricstore.MetricPingLatency]
	byTask := func(points []metric.AggregatePoint) map[string][]metric.AggregatePoint {
		out := make(map[string][]metric.AggregatePoint)
		for _, point := range points {
			taskID := point.Tags["task_id"]
			if _, ok := assigned[taskID]; ok {
				out[taskID] = append(out[taskID], point)
			}
		}
		return out
	}
	avg, minimum, maximum := byTask(latency[metric.AggAvg]), byTask(latency[metric.AggMin]), byTask(latency[metric.AggMax])
	last, p50, p99 := byTask(latency[metric.AggLast]), byTask(latency[metric.AggP50]), byTask(latency[metric.AggP99])
	loss := byTask(loaded.Values[metricstore.MetricPingLoss][metric.AggAvg])
	result := make(map[string]pingStat)
	for taskID, task := range assigned {
		var latencySum, lossCount float64
		total := 0
		for _, point := range avg[taskID] {
			total += point.Count
			latencySum += point.Value * float64(point.Count)
		}
		for _, point := range loss[taskID] {
			lossCount += math.Max(0, math.Min(1, point.Value)) * float64(point.Count)
		}
		if total == 0 {
			continue
		}
		lossCount = math.Min(float64(total), lossCount)
		valid := float64(total) - lossCount
		stat := pingStat{Name: task.Name, Weight: task.Weight, Latest: -1, Min: -1, Loss: lossCount / float64(total) * 100}
		if valid > 0 {
			// Every failed latency sample is -1 and has a matching loss=1.
			// Undo its contribution before dividing by the successful count.
			stat.Avg = int(math.Round((latencySum + lossCount) / valid))
		}
		if value := positiveAggregateMin(minimum[taskID]); value != nil {
			stat.Min = int(math.Round(*value))
		}
		if value := positiveAggregateMax(maximum[taskID]); value != nil {
			stat.Max = int(math.Round(*value))
		}
		var latest time.Time
		for _, point := range last[taskID] {
			if latest.IsZero() || point.Bucket.After(latest) {
				latest, stat.Latest = point.Bucket, int(math.Round(point.Value))
			}
		}
		median, _ := weightedAggregateValue(p50[taskID], true)
		upper, _ := weightedAggregateValue(p99[taskID], true)
		if median != nil && upper != nil && *median > 0 && *upper >= *median {
			stat.Tail = (*upper - *median) / *median
		}
		// Percentiles merged from buckets are estimates; mixed-loss minima
		// cannot recover a successful minimum without the raw observations.
		stat.Approximate = true
		result[taskID] = stat
	}
	return result, nil
}

func init() {
	RegisterWithGroupAndMeta("getNodes", "common",
		func(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
			return getNodes(ctx, req)
		},
		&rpc.MethodMeta{
			Name:    "getNodes",
			Summary: "Get all nodes",
			Params: []rpc.ParamMeta{
				{
					Name:        "uuid",
					Description: "Specify the UUID of the node",
					Required:    false,
					Type:        "string",
				},
			},
			Returns: "Client | { [uuid]: Client }",
		},
	)
	RegisterWithGroupAndMeta("getNodesLatestStatus", "common",
		func(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
			return getNodesLatestStatus(ctx, req)
		},
		&rpc.MethodMeta{
			Name:    "getNodesLatestStatus",
			Summary: "Get latest status reports (single node or map).",
			Params: []rpc.ParamMeta{
				{
					Name:        "uuid",
					Description: "Specify the UUID of the node (optional)",
					Required:    false,
					Type:        "string",
				},
				{
					Name:        "uuids",
					Description: "Specify multiple UUIDs (array) to get subset (ignored if uuid provided)",
					Required:    false,
					Type:        "string[]",
				},
			},
			Returns: "Record | { [uuid]: Record }",
		},
	)
	Register("getPublicInfo", getPublicInfo)
	Register("getVersion", getVersion)
	Register("getNodeRecentStatus", getNodeRecentStatus)
}

func getNodes(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	cinfo, err := clients.GetAllClientBasicInfo()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to get client info", cinfo)
	}
	meta := rpc.MetaFromContext(ctx)

	if meta.Principal == nil || !meta.Principal.HasRole(rpc.RoleAdmin) {
		// 过滤 Hidden 节点并隐藏敏感字段
		filtered := make([]models.Client, 0, len(cinfo))
		for _, node := range cinfo {
			if node.Hidden { // 非 admin 不显示隐藏节点
				continue
			}
			node.IPv4 = ""
			node.IPv6 = ""

			node.Remark = ""
			node.Version = ""
			node.Token = ""
			filtered = append(filtered, node)
		}
		cinfo = filtered
	}
	if params.UUID != "" {
		for _, node := range cinfo {
			if node.UUID == params.UUID {
				return node, nil
			}
		}
		return nil, rpc.MakeError(rpc.InvalidParams, "Node not found", params.UUID)
	}

	// 返回以 uuid 为键的字典（每个 value 自身也包含 uuid 字段）
	nodeMap := make(map[string]models.Client, len(cinfo))
	for _, node := range cinfo {
		nodeMap[node.UUID] = node
	}
	return nodeMap, nil
}

func gpuUsageFromReport(rep *v2.Report) float32 {
	if rep == nil || rep.GPU == nil {
		return 0
	}
	return float32(rep.GPU.AverageUsage)
}

func getPublicInfo(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	info, err := database.GetPublicInfo()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to get public info", err.Error())
	}
	return info, nil
}

func getNodesLatestStatus(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID  string   `json:"uuid"`
		UUIDs []string `json:"uuids"`
	}
	req.BindParams(&params)

	meta := rpc.MetaFromContext(ctx)
	latest := agent_runtime.GetLatestReport()
	onlineUUIDs := agent_runtime.GetAllOnlineUUIDs()
	onlineSet := make(map[string]bool, len(onlineUUIDs))
	for _, uuid := range onlineUUIDs {
		onlineSet[uuid] = true
	}

	// Hidden 过滤
	if meta.Principal == nil || !meta.Principal.HasRole(rpc.RoleAdmin) {
		cinfo, err := clients.GetAllClientBasicInfo()
		if err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to get client info", err.Error())
		}
		hidden := make(map[string]bool, len(cinfo))
		for _, c := range cinfo {
			if c.Hidden {
				hidden[c.UUID] = true
			}
		}
		for uuid := range latest {
			if hidden[uuid] {
				delete(latest, uuid)
			}
		}
	}

	// 如果指定 uuid 但找不到，直接返回 not found
	if params.UUID != "" {
		if _, ok := latest[params.UUID]; !ok {
			return nil, rpc.MakeError(rpc.InvalidParams, "Node not found", params.UUID)
		}
	}

	type recordLike struct {
		Client          string              `json:"client"`
		Time            time.Time           `json:"time"`
		Cpu             float32             `json:"cpu"`
		Gpu             float32             `json:"gpu"`
		GpuCount        int                 `json:"gpu_count,omitempty"`
		GpuAverageUsage float64             `json:"gpu_average_usage,omitempty"`
		GpuDetailedInfo []v2.GPUDeviceInfo  `json:"gpu_detailed_info,omitempty"`
		Ram             int64               `json:"ram"`
		RamTotal        int64               `json:"ram_total"`
		Swap            int64               `json:"swap"`
		SwapTotal       int64               `json:"swap_total"`
		Load            float32             `json:"load"`
		Load5           float32             `json:"load5"`
		Load15          float32             `json:"load15"`
		Temp            float32             `json:"temp"`
		Disk            int64               `json:"disk"`
		DiskTotal       int64               `json:"disk_total"`
		NetIn           int64               `json:"net_in"`
		NetOut          int64               `json:"net_out"`
		NetTotalUp      int64               `json:"net_total_up"`
		NetTotalDown    int64               `json:"net_total_down"`
		Process         int                 `json:"process"`
		Connections     int                 `json:"connections"`
		ConnectionsUdp  int                 `json:"connections_udp"`
		Online          bool                `json:"online"`
		Uptime          int64               `json:"uptime"`
		Ping            map[string]pingStat `json:"ping"`
	}

	respMap := make(map[string]recordLike, len(latest))

	// 预取所有 ping 任务
	pingTasks, _ := tasks.GetAllPingTasks()

	appendOne := func(uuid string, rep *v2.Report) {
		if rep == nil {
			return
		}
		stats := getPingStatsForNodeContext(ctx, uuid, pingTasks)
		rl := recordLike{
			Client:         uuid,
			Time:           rep.UpdatedAt,
			Cpu:            float32(rep.CPU.Usage),
			Gpu:            gpuUsageFromReport(rep),
			Ram:            rep.Ram.Used,
			RamTotal:       rep.Ram.Total,
			Swap:           rep.Swap.Used,
			SwapTotal:      rep.Swap.Total,
			Load:           float32(rep.Load.Load1),
			Load5:          float32(rep.Load.Load5),
			Load15:         float32(rep.Load.Load15),
			Temp:           0,
			Disk:           rep.Disk.Used,
			DiskTotal:      rep.Disk.Total,
			NetIn:          rep.Network.Down,
			NetOut:         rep.Network.Up,
			NetTotalUp:     rep.Network.TotalUp,
			NetTotalDown:   rep.Network.TotalDown,
			Process:        rep.Process,
			Connections:    rep.Connections.TCP + rep.Connections.UDP,
			ConnectionsUdp: rep.Connections.UDP,
			Online:         onlineSet[uuid],
			Uptime:         rep.Uptime,
			Ping:           stats,
		}
		if rep.GPU != nil {
			rl.GpuCount = rep.GPU.Count
			rl.GpuAverageUsage = rep.GPU.AverageUsage
			rl.GpuDetailedInfo = rep.GPU.DetailedInfo
		}
		respMap[uuid] = rl
	}

	// 选择逻辑
	if params.UUID != "" { // 单个
		appendOne(params.UUID, latest[params.UUID])
		return respMap[params.UUID], nil
	}
	selected := map[string]bool{}
	if len(params.UUIDs) > 0 {
		for _, id := range params.UUIDs {
			selected[id] = true
		}
		for uuid, rep := range latest {
			if selected[uuid] {
				appendOne(uuid, rep)
			}
		}
		return respMap, nil
	}
	for uuid, rep := range latest {
		appendOne(uuid, rep)
	}
	return respMap, nil
}

func getVersion(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return struct {
		Version string `json:"version"`
		Hash    string `json:"hash"`
	}{
		Version: utils.CurrentVersion,
		Hash:    utils.VersionHash,
	}, nil
}

func getNodeRecentStatus(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		UUID string `json:"uuid"`
	}
	req.BindParams(&params)
	if params.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "UUID is required", params)
	}
	meta := rpc.MetaFromContext(ctx)
	// 登录状态检查
	isLogin := false
	if meta.Principal != nil && meta.Principal.HasRole(rpc.RoleAdmin) {
		isLogin = true
	}

	// 仅在未登录时需要 Hidden 信息做过滤
	hiddenMap := map[string]bool{}
	if !isLogin {
		var hiddenClients []models.Client
		db := dbcore.GetDBInstance()
		_ = db.Select("uuid").Where("hidden = ?", true).Find(&hiddenClients).Error
		for _, cli := range hiddenClients {
			hiddenMap[cli.UUID] = true
		}

		if hiddenMap[params.UUID] {
			return nil, rpc.MakeError(rpc.InvalidParams, "UUID is required", params) //防止未登录用户获取隐藏客户端数据
		}
	}

	reports := agent_runtime.GetRecentReports(params.UUID)

	// 扁平化为 { count, records: [] }
	type flatRecord struct {
		Client         string    `json:"client"`
		Time           time.Time `json:"time"`
		Cpu            float32   `json:"cpu"`
		Gpu            float32   `json:"gpu"`
		Ram            int64     `json:"ram"`
		RamTotal       int64     `json:"ram_total"`
		Swap           int64     `json:"swap"`
		SwapTotal      int64     `json:"swap_total"`
		Load           float32   `json:"load"`
		Temp           float32   `json:"temp"`
		Disk           int64     `json:"disk"`
		DiskTotal      int64     `json:"disk_total"`
		NetIn          int64     `json:"net_in"`
		NetOut         int64     `json:"net_out"`
		NetTotalUp     int64     `json:"net_total_up"`
		NetTotalDown   int64     `json:"net_total_down"`
		Process        int       `json:"process"`
		Connections    int       `json:"connections"`
		ConnectionsUdp int       `json:"connections_udp"`
	}

	resp := struct {
		Count   int          `json:"count"`
		Records []flatRecord `json:"records"`
	}{
		Count:   0,
		Records: []flatRecord{},
	}

	if len(reports) == 0 {
		return resp, nil
	}

	resp.Records = make([]flatRecord, 0, len(reports))
	for _, r := range reports {
		fr := flatRecord{
			Client:         params.UUID,
			Time:           r.UpdatedAt,
			Cpu:            float32(r.CPU.Usage),
			Gpu:            gpuUsageFromReport(&r),
			Ram:            r.Ram.Used,
			RamTotal:       r.Ram.Total,
			Swap:           r.Swap.Used,
			SwapTotal:      r.Swap.Total,
			Load:           float32(r.Load.Load1),
			Temp:           0,
			Disk:           r.Disk.Used,
			DiskTotal:      r.Disk.Total,
			NetIn:          r.Network.Down,
			NetOut:         r.Network.Up,
			NetTotalUp:     r.Network.TotalUp,
			NetTotalDown:   r.Network.TotalDown,
			Process:        r.Process,
			Connections:    r.Connections.TCP + r.Connections.UDP,
			ConnectionsUdp: r.Connections.UDP,
		}
		resp.Records = append(resp.Records, fr)
	}
	resp.Count = len(resp.Records)
	return resp, nil
}
