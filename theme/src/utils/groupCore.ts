/** Node-group helpers shared by the public theme and the admin console. */

export function parseNodeGroups(group: string | null | undefined): string[] {
  if (!group)
    return []

  const seen = new Set<string>()
  const groups: string[] = []
  for (const item of group.split(';')) {
    const normalized = item.trim()
    if (!normalized || seen.has(normalized))
      continue
    seen.add(normalized)
    groups.push(normalized)
  }
  return groups
}

export function isNodeInGroup(
  group: string | null | undefined,
  selectedGroup: string,
): boolean {
  return selectedGroup === 'all' || parseNodeGroups(group).includes(selectedGroup)
}

export function isNodeUngrouped(group: string | null | undefined): boolean {
  return parseNodeGroups(group).length === 0
}
