import { getCoordByCode, getCountryCodeFromRegion } from '@/utils/geoHelper'

export interface RegionNodeLike {
  region?: string | null
  online?: boolean | null
  net_in?: number | null
  net_out?: number | null
}

export interface RegionNodeAggregate {
  code: string
  coord: [number, number]
  servers: number
  onlineServers: number
  offlineServers: number
  up: number
  down: number
}

/** Aggregate nodes by mappable ISO region for every geographic visualization. */
export function aggregateNodesByRegion(
  nodes: readonly RegionNodeLike[],
): RegionNodeAggregate[] {
  const aggregates = new Map<string, RegionNodeAggregate>()

  for (const node of nodes) {
    const code = getCountryCodeFromRegion(node.region)
    if (!code)
      continue
    const coord = getCoordByCode(code)
    if (!coord)
      continue

    let aggregate = aggregates.get(code)
    if (!aggregate) {
      aggregate = {
        code,
        coord,
        servers: 0,
        onlineServers: 0,
        offlineServers: 0,
        up: 0,
        down: 0,
      }
      aggregates.set(code, aggregate)
    }

    aggregate.servers += 1
    if (node.online) {
      aggregate.onlineServers += 1
      aggregate.up += Number(node.net_out) || 0
      aggregate.down += Number(node.net_in) || 0
    }
    else {
      aggregate.offlineServers += 1
    }
  }

  return Array.from(aggregates.values()).sort(
    (left, right) => right.onlineServers - left.onlineServers || right.servers - left.servers,
  )
}

export function countNodeAvailability(nodes: readonly Pick<RegionNodeLike, 'online'>[]) {
  const totalServers = nodes.length
  let onlineServers = 0
  for (const node of nodes) {
    if (node.online)
      onlineServers += 1
  }
  return {
    totalServers,
    onlineServers,
    offlineServers: totalServers - onlineServers,
  }
}
