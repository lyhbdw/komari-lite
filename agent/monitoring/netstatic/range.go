package netstatic

import "container/heap"

type pendingTraffic struct {
	name string
	TrafficData
}
type futureTraffic []pendingTraffic

func (h futureTraffic) Len() int            { return len(h) }
func (h futureTraffic) Less(i, j int) bool  { return h[i].Timestamp < h[j].Timestamp }
func (h futureTraffic) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *futureTraffic) Push(v interface{}) { *h = append(*h, v.(pendingTraffic)) }
func (h *futureTraffic) Pop() interface{} {
	a := *h
	v := a[len(a)-1]
	a[len(a)-1] = pendingTraffic{}
	*h = a[:len(a)-1]
	return v
}

type trafficRangeCache struct {
	valid      bool
	start, end uint64
	totals     map[string]TrafficData
	future     futureTraffic
	builds     uint64
}

var rangeCache trafficRangeCache

func invalidateRangeLocked() { rangeCache.valid = false }

func (c *trafficRangeCache) add(name string, td TrafficData) {
	if (c.start != 0 && td.Timestamp < c.start) || (td.Tx == 0 && td.Rx == 0) {
		return
	}
	if c.end != 0 && td.Timestamp > c.end {
		heap.Push(&c.future, pendingTraffic{name, td})
		return
	}
	total := c.totals[name]
	total.Tx += td.Tx
	total.Rx += td.Rx
	c.totals[name] = total
}

func appendTrafficLocked(name string, td TrafficData) {
	staticCache[name] = append(staticCache[name], td)
	if rangeCache.valid {
		rangeCache.add(name, td)
	}
}

func trafficRangeLocked(start, end uint64) map[string]TrafficData {
	c := &rangeCache
	if !c.valid || c.start != start || (end != 0 && (c.end == 0 || end < c.end)) {
		builds := c.builds + 1
		*c = trafficRangeCache{valid: true, start: start, end: end, totals: map[string]TrafficData{}, builds: builds}
		for name, arr := range store.Interfaces {
			for _, td := range arr {
				c.add(name, td)
			}
		}
		for name, arr := range staticCache {
			for _, td := range arr {
				c.add(name, td)
			}
		}
	} else {
		c.end = end
		for len(c.future) > 0 && (end == 0 || c.future[0].Timestamp <= end) {
			next := heap.Pop(&c.future).(pendingTraffic)
			c.add(next.name, next.TrafficData)
		}
	}
	result := make(map[string]TrafficData, len(c.totals))
	for name, td := range c.totals {
		result[name] = td
	}
	return result
}
