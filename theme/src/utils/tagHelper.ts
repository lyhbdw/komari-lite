export * from './billingCore'

/** 支持的标签颜色 */
type TagColor
  = | 'ruby'
    | 'gray'
    | 'gold'
    | 'bronze'
    | 'brown'
    | 'yellow'
    | 'amber'
    | 'orange'
    | 'tomato'
    | 'red'
    | 'crimson'
    | 'pink'
    | 'plum'
    | 'purple'
    | 'violet'
    | 'iris'
    | 'indigo'
    | 'blue'
    | 'cyan'
    | 'teal'
    | 'jade'
    | 'green'
    | 'grass'
    | 'lime'
    | 'mint'
    | 'sky'

/** 所有支持的标签颜色列表 */
const TAG_COLORS = [
  'ruby',
  'gray',
  'gold',
  'bronze',
  'brown',
  'yellow',
  'amber',
  'orange',
  'tomato',
  'red',
  'crimson',
  'pink',
  'plum',
  'purple',
  'violet',
  'iris',
  'indigo',
  'blue',
  'cyan',
  'teal',
  'jade',
  'green',
  'grass',
  'lime',
  'mint',
  'sky',
] as const

/** Radix Themes 颜色到 HEX 的映射（基于 light 模式的 9 色阶） */
const TAG_COLOR_HEX_MAP: Record<TagColor, string> = {
  ruby: '#E5484D',
  gray: '#8D8D8D',
  gold: '#E5C00D',
  bronze: '#C2853C',
  brown: '#AA6A38',
  yellow: '#F9D400',
  amber: '#F5B21A',
  orange: '#F97316',
  tomato: '#E54D2E',
  red: '#E5484D',
  crimson: '#E93D82',
  pink: '#E24D8C',
  plum: '#A855C2',
  purple: '#8E4EC6',
  violet: '#7C5DFA',
  iris: '#5B5BD6',
  indigo: '#6366F1',
  blue: '#0090FF',
  cyan: '#00A2C7',
  teal: '#12A594',
  jade: '#29A383',
  green: '#30A46C',
  grass: '#46A358',
  lime: '#84CC16',
  mint: '#4FD1C5',
  sky: '#00A6ED',
}

const TAG_COLOR_SUFFIX_REGEX = /<(\w+)>$/
const TAG_COLOR_SUFFIX_REMOVE_REGEX = /<\w+>$/

function parseTagWithColor(tag: string): { text: string, color: TagColor | null } {
  const colorMatch = tag.match(TAG_COLOR_SUFFIX_REGEX)
  if (colorMatch && colorMatch[1]) {
    const colorCandidate = colorMatch[1].toLowerCase()
    const text = tag.replace(TAG_COLOR_SUFFIX_REMOVE_REGEX, '')
    if ((TAG_COLORS as readonly string[]).includes(colorCandidate)) {
      return { text, color: colorCandidate as TagColor }
    }
  }
  return { text: tag, color: null }
}

function getTagColorHex(color: TagColor): string {
  return TAG_COLOR_HEX_MAP[color]
}

/**
 * 解析标签字符串为标签列表
 */
export function parseTags(tags: string | undefined): Array<{ text: string, color: TagColor, hex: string }> {
  if (!tags || tags.trim() === '')
    return []

  const tagList = tags.split(';').filter(tag => tag.trim() !== '')

  return tagList.map((tag, index) => {
    const { text, color } = parseTagWithColor(tag)
    const defaultColor = TAG_COLORS[index % TAG_COLORS.length] ?? 'blue'
    const resolvedColor = color ?? defaultColor
    return {
      text,
      color: resolvedColor,
      hex: getTagColorHex(resolvedColor),
    }
  })
}
