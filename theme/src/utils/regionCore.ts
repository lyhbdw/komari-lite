export interface RegionInfo {
  en: string
  zh: string
  code: string
  aliases: string[]
}

type CompactRegionTuple = readonly [en: string, zh: string, code: string, extraAliases?: readonly string[]]

const COMPACT_REGION_DATA: Record<string, CompactRegionTuple> = {
  '🇭🇰': ['Hong Kong', '香港', 'HK'],
  '🇨🇳': ['China', '中国', 'CN', ['中华人民共和国', 'prc']],
  '🇺🇸': ['United States', '美国', 'US', ['usa', 'america', '美利坚']],
  '🇯🇵': ['Japan', '日本', 'JP'],
  '🇰🇷': ['South Korea', '韩国', 'KR', ['korea', '南韩']],
  '🇸🇬': ['Singapore', '新加坡', 'SG'],
  '🇹🇼': ['Taiwan', '台湾', 'TW', ['台灣']],
  '🇬🇧': ['United Kingdom', '英国', 'GB', ['uk', 'britain', '英國']],
  '🇩🇪': ['Germany', '德国', 'DE', ['deutschland', '德國']],
  '🇫🇷': ['France', '法国', 'FR', ['法國']],
  '🇨🇦': ['Canada', '加拿大', 'CA'],
  '🇦🇺': ['Australia', '澳大利亚', 'AU', ['澳洲']],
  '🇷🇺': ['Russia', '俄罗斯', 'RU', ['俄國']],
  '🇮🇳': ['India', '印度', 'IN'],
  '🇧🇷': ['Brazil', '巴西', 'BR'],
  '🇳🇱': ['Netherlands', '荷兰', 'NL', ['holland', '荷蘭']],
  '🇮🇹': ['Italy', '意大利', 'IT'],
  '🇪🇸': ['Spain', '西班牙', 'ES'],
  '🇸🇪': ['Sweden', '瑞典', 'SE'],
  '🇳🇴': ['Norway', '挪威', 'NO'],
  '🇫🇮': ['Finland', '芬兰', 'FI', ['芬蘭']],
  '🇨🇭': ['Switzerland', '瑞士', 'CH'],
  '🇦🇹': ['Austria', '奥地利', 'AT', ['奧地利']],
  '🇧🇪': ['Belgium', '比利时', 'BE', ['比利時']],
  '🇵🇹': ['Portugal', '葡萄牙', 'PT'],
  '🇬🇷': ['Greece', '希腊', 'GR', ['希臘']],
  '🇹🇷': ['Turkey', '土耳其', 'TR'],
  '🇵🇱': ['Poland', '波兰', 'PL', ['波蘭']],
  '🇨🇿': ['Czech Republic', '捷克', 'CZ', ['czech']],
  '🇭🇺': ['Hungary', '匈牙利', 'HU'],
  '🇷🇴': ['Romania', '罗马尼亚', 'RO', ['羅馬尼亞']],
  '🇧🇬': ['Bulgaria', '保加利亚', 'BG', ['保加利亞']],
  '🇭🇷': ['Croatia', '克罗地亚', 'HR', ['克羅地亞']],
  '🇸🇮': ['Slovenia', '斯洛文尼亚', 'SI', ['斯洛文尼亞']],
  '🇸🇰': ['Slovakia', '斯洛伐克', 'SK'],
  '🇱🇻': ['Latvia', '拉脱维亚', 'LV', ['拉脫維亞']],
  '🇱🇹': ['Lithuania', '立陶宛', 'LT'],
  '🇪🇪': ['Estonia', '爱沙尼亚', 'EE', ['愛沙尼亞']],
  '🇲🇽': ['Mexico', '墨西哥', 'MX'],
  '🇦🇷': ['Argentina', '阿根廷', 'AR'],
  '🇨🇱': ['Chile', '智利', 'CL'],
  '🇨🇴': ['Colombia', '哥伦比亚', 'CO', ['哥倫比亞']],
  '🇵🇪': ['Peru', '秘鲁', 'PE', ['秘魯']],
  '🇻🇪': ['Venezuela', '委内瑞拉', 'VE', ['委內瑞拉']],
  '🇺🇾': ['Uruguay', '乌拉圭', 'UY', ['烏拉圭']],
  '🇪🇨': ['Ecuador', '厄瓜多尔', 'EC', ['厄瓜多爾']],
  '🇧🇴': ['Bolivia', '玻利维亚', 'BO', ['玻利維亞']],
  '🇵🇾': ['Paraguay', '巴拉圭', 'PY'],
  '🇬🇾': ['Guyana', '圭亚那', 'GY', ['圭亞那']],
  '🇸🇷': ['Suriname', '苏里南', 'SR', ['蘇里南']],
  '🇫🇰': ['Falkland Islands', '福克兰群岛', 'FK', ['falkland', '福克兰', '福克蘭']],
  '🇬🇫': ['French Guiana', '法属圭亚那', 'GF', ['法屬圭亞那']],
  '🇵🇦': ['Panama', '巴拿马', 'PA', ['巴拿馬']],
  '🇨🇷': ['Costa Rica', '哥斯达黎加', 'CR', ['哥斯達黎加']],
  '🇳🇮': ['Nicaragua', '尼加拉瓜', 'NI'],
  '🇭🇳': ['Honduras', '洪都拉斯', 'HN'],
  '🇬🇹': ['Guatemala', '危地马拉', 'GT', ['危地馬拉']],
  '🇧🇿': ['Belize', '伯利兹', 'BZ', ['伯利茲']],
  '🇸🇻': ['El Salvador', '萨尔瓦多', 'SV', ['薩爾瓦多']],
  '🇯🇲': ['Jamaica', '牙买加', 'JM', ['牙買加']],
  '🇨🇺': ['Cuba', '古巴', 'CU'],
  '🇩🇴': ['Dominican Republic', '多明尼加', 'DO', ['dominican']],
  '🇭🇹': ['Haiti', '海地', 'HT'],
  '🇧🇸': ['Bahamas', '巴哈马', 'BS', ['巴哈馬']],
  '🇧🇧': ['Barbados', '巴巴多斯', 'BB'],
  '🇹🇹': ['Trinidad and Tobago', '特立尼达和多巴哥', 'TT', ['trinidad', '特立尼达', '特立尼達']],
  '🇵🇭': ['Philippines', '菲律宾', 'PH', ['菲律賓']],
  '🇹🇭': ['Thailand', '泰国', 'TH', ['泰國']],
  '🇻🇳': ['Vietnam', '越南', 'VN'],
  '🇲🇾': ['Malaysia', '马来西亚', 'MY', ['馬來西亞']],
  '🇮🇩': ['Indonesia', '印度尼西亚', 'ID', ['印尼']],
  '🇱🇦': ['Laos', '老挝', 'LA', ['老撾']],
  '🇰🇭': ['Cambodia', '柬埔寨', 'KH'],
  '🇲🇲': ['Myanmar', '缅甸', 'MM', ['burma', '緬甸']],
  '🇧🇳': ['Brunei', '文莱', 'BN', ['汶萊']],
  '🇪🇬': ['Egypt', '埃及', 'EG'],
  '🇿🇦': ['South Africa', '南非', 'ZA'],
  '🇳🇬': ['Nigeria', '尼日利亚', 'NG', ['尼日利亞']],
  '🇰🇪': ['Kenya', '肯尼亚', 'KE', ['肯亞']],
  '🇪🇹': ['Ethiopia', '埃塞俄比亚', 'ET', ['埃塞俄比亞']],
  '🇬🇭': ['Ghana', '加纳', 'GH', ['迦納']],
  '🇺🇬': ['Uganda', '乌干达', 'UG', ['烏干達']],
  '🇹🇿': ['Tanzania', '坦桑尼亚', 'TZ', ['坦尚尼亞']],
  '🇷🇼': ['Rwanda', '卢旺达', 'RW', ['盧旺達']],
  '🇿🇼': ['Zimbabwe', '津巴布韦', 'ZW', ['辛巴威']],
  '🇿🇲': ['Zambia', '赞比亚', 'ZM', ['尚比亞']],
  '🇧🇼': ['Botswana', '博茨瓦纳', 'BW', ['波札那']],
  '🇳🇦': ['Namibia', '纳米比亚', 'NA', ['納米比亞']],
  '🇲🇦': ['Morocco', '摩洛哥', 'MA'],
  '🇩🇿': ['Algeria', '阿尔及利亚', 'DZ', ['阿爾及利亞']],
  '🇹🇳': ['Tunisia', '突尼斯', 'TN'],
  '🇱🇾': ['Libya', '利比亚', 'LY', ['利比亞']],
  '🇸🇩': ['Sudan', '苏丹', 'SD', ['蘇丹']],
  '🇸🇸': ['South Sudan', '南苏丹', 'SS', ['南蘇丹']],
  '🇨🇩': ['Democratic Republic of Congo', '刚果民主共和国', 'CD', ['congo', 'drc', '刚果', '剛果']],
  '🇨🇬': ['Republic of Congo', '刚果共和国', 'CG', ['congo', '刚果', '剛果']],
  '🇨🇫': ['Central African Republic', '中非共和国', 'CF', ['central african', '中非']],
  '🇨🇲': ['Cameroon', '喀麦隆', 'CM', ['喀麥隆']],
  '🇹🇩': ['Chad', '乍得', 'TD'],
  '🇳🇪': ['Niger', '尼日尔', 'NE', ['尼日爾']],
  '🇲🇱': ['Mali', '马里', 'ML', ['馬利']],
  '🇧🇫': ['Burkina Faso', '布基纳法索', 'BF', ['burkina', '布吉納法索']],
  '🇸🇳': ['Senegal', '塞内加尔', 'SN', ['塞內加爾']],
  '🇬🇲': ['Gambia', '冈比亚', 'GM', ['甘比亞']],
  '🇬🇼': ['Guinea-Bissau', '几内亚比绍', 'GW', ['幾內亞比索']],
  '🇬🇳': ['Guinea', '几内亚', 'GN', ['幾內亞']],
  '🇸🇱': ['Sierra Leone', '塞拉利昂', 'SL'],
  '🇱🇷': ['Liberia', '利比里亚', 'LR', ['賴比瑞亞']],
  '🇨🇮': ['Ivory Coast', '科特迪瓦', 'CI', ['象牙海岸']],
  '🇹🇬': ['Togo', '多哥', 'TG'],
  '🇧🇯': ['Benin', '贝宁', 'BJ', ['貝寧']],
  '🇦🇨': ['AC', 'AC', 'AC'],
  '🇦🇩': ['AD', 'AD', 'AD'],
  '🇦🇪': ['United Arab Emirates', '阿联酋', 'AE'],
  '🇦🇫': ['Afghanistan', '阿富汗', 'AF'],
  '🇦🇬': ['Antigua and Barbuda', '安提瓜和巴布达', 'AG'],
  '🇦🇮': ['Anguilla', '安圭拉', 'AI'],
  '🇦🇱': ['Albania', '阿尔巴尼亚', 'AL'],
  '🇦🇲': ['Armenia', '亚美尼亚', 'AM'],
  '🇦🇴': ['Angola', '安哥拉', 'AO'],
  '🇦🇶': ['Antarctica', '南极洲', 'AQ'],
  '🇦🇸': ['American Samoa', '美属萨摩亚', 'AS'],
  '🇦🇼': ['Aruba', '阿鲁巴', 'AW'],
  '🇦🇽': ['Åland Islands', '奥兰群岛', 'AX'],
  '🇦🇿': ['Azerbaijan', '阿塞拜疆', 'AZ'],
  '🇧🇦': ['Bosnia and Herzegovina', '波黑', 'BA'],
  '🇧🇩': ['Bangladesh', '孟加拉国', 'BD'],
  '🇧🇭': ['Bahrain', '巴林', 'BH'],
  '🇧🇮': ['Burundi', '布隆迪', 'BI'],
  '🇧🇱': ['Saint Barthélemy', '圣巴泰勒米', 'BL'],
  '🇧🇲': ['Bermuda', '百慕大', 'BM'],
  '🇧🇶': ['Caribbean Netherlands', '荷属加勒比', 'BQ'],
  '🇧🇹': ['Bhutan', '不丹', 'BT'],
  '🇧🇻': ['Bouvet Island', '布韦岛', 'BV'],
  '🇧🇾': ['Belarus', '白俄罗斯', 'BY'],
  '🇨🇨': ['Cocos Islands', '科科斯群岛', 'CC'],
  '🇨🇰': ['Cook Islands', '库克群岛', 'CK'],
  '🇨🇵': ['Clipperton Island', '克利珀顿岛', 'CP'],
  '🇨🇻': ['Cape Verde', '佛得角', 'CV'],
  '🇨🇼': ['Curaçao', '库拉索', 'CW'],
  '🇨🇽': ['Christmas Island', '圣诞岛', 'CX'],
  '🇨🇾': ['Cyprus', '塞浦路斯', 'CY'],
  '🇩🇬': ['Diego Garcia', '迪戈加西亚', 'DG'],
  '🇩🇯': ['Djibouti', '吉布提', 'DJ'],
  '🇩🇰': ['Denmark', '丹麦', 'DK'],
  '🇩🇲': ['Dominica', '多米尼克', 'DM'],
  '🇪🇦': ['Ceuta and Melilla', '休达和梅利利亚', 'EA'],
  '🇪🇭': ['Western Sahara', '西撒哈拉', 'EH'],
  '🇪🇷': ['Eritrea', '厄立特里亚', 'ER'],
  '🇪🇺': ['European Union', '欧盟', 'EU'],
  '🇫🇯': ['Fiji', '斐济', 'FJ'],
  '🇫🇲': ['Micronesia', '密克罗尼西亚', 'FM'],
  '🇫🇴': ['Faroe Islands', '法罗群岛', 'FO'],
  '🇬🇦': ['Gabon', '加蓬', 'GA'],
  '🇬🇩': ['Grenada', '格林纳达', 'GD'],
  '🇬🇪': ['Georgia', '格鲁吉亚', 'GE'],
  '🇬🇬': ['Guernsey', '根西岛', 'GG'],
  '🇬🇮': ['Gibraltar', '直布罗陀', 'GI'],
  '🇬🇱': ['Greenland', '格陵兰', 'GL'],
  '🇬🇵': ['Guadeloupe', '瓜德罗普', 'GP'],
  '🇬🇶': ['Equatorial Guinea', '赤道几内亚', 'GQ'],
  '🇬🇸': ['South Georgia', '南乔治亚', 'GS'],
  '🇬🇺': ['Guam', '关岛', 'GU'],
  '🇭🇲': ['Heard Island', '赫德岛', 'HM'],
  '🇮🇨': ['Canary Islands', '加那利群岛', 'IC'],
  '🇮🇪': ['Ireland', '爱尔兰', 'IE'],
  '🇮🇱': ['Israel', '以色列', 'IL'],
  '🇮🇲': ['Isle of Man', '马恩岛', 'IM'],
  '🇮🇴': ['British Indian Ocean Territory', '英属印度洋领地', 'IO'],
  '🇮🇶': ['Iraq', '伊拉克', 'IQ'],
  '🇮🇷': ['Iran', '伊朗', 'IR'],
  '🇮🇸': ['Iceland', '冰岛', 'IS'],
  '🇯🇪': ['Jersey', '泽西岛', 'JE'],
  '🇯🇴': ['Jordan', '约旦', 'JO'],
  '🇰🇬': ['Kyrgyzstan', '吉尔吉斯斯坦', 'KG'],
  '🇰🇮': ['Kiribati', '基里巴斯', 'KI'],
  '🇰🇲': ['Comoros', '科摩罗', 'KM'],
  '🇰🇳': ['Saint Kitts and Nevis', '圣基茨和尼维斯', 'KN'],
  '🇰🇵': ['North Korea', '朝鲜', 'KP'],
  '🇰🇼': ['Kuwait', '科威特', 'KW'],
  '🇰🇾': ['Cayman Islands', '开曼群岛', 'KY'],
  '🇰🇿': ['Kazakhstan', '哈萨克斯坦', 'KZ'],
  '🇱🇧': ['Lebanon', '黎巴嫩', 'LB'],
  '🇱🇨': ['Saint Lucia', '圣卢西亚', 'LC'],
  '🇱🇮': ['Liechtenstein', '列支敦士登', 'LI'],
  '🇱🇰': ['Sri Lanka', '斯里兰卡', 'LK'],
  '🇱🇸': ['Lesotho', '莱索托', 'LS'],
  '🇱🇺': ['Luxembourg', '卢森堡', 'LU'],
  '🇲🇨': ['Monaco', '摩纳哥', 'MC'],
  '🇲🇩': ['Moldova', '摩尔多瓦', 'MD'],
  '🇲🇪': ['Montenegro', '黑山', 'ME'],
  '🇲🇫': ['Saint Martin', '法属圣马丁', 'MF'],
  '🇲🇬': ['Madagascar', '马达加斯加', 'MG'],
  '🇲🇭': ['Marshall Islands', '马绍尔群岛', 'MH'],
  '🇲🇰': ['North Macedonia', '北马其顿', 'MK'],
  '🇲🇳': ['Mongolia', '蒙古', 'MN'],
  '🇲🇴': ['Macau', '澳门', 'MO'],
  '🇲🇵': ['Northern Mariana Islands', '北马里亚纳群岛', 'MP'],
  '🇲🇶': ['Martinique', '马提尼克', 'MQ'],
  '🇲🇷': ['Mauritania', '毛里塔尼亚', 'MR'],
  '🇲🇸': ['Montserrat', '蒙特塞拉特', 'MS'],
  '🇲🇹': ['Malta', '马耳他', 'MT'],
  '🇲🇺': ['Mauritius', '毛里求斯', 'MU'],
  '🇲🇻': ['Maldives', '马尔代夫', 'MV'],
  '🇲🇼': ['Malawi', '马拉维', 'MW'],
  '🇲🇿': ['Mozambique', '莫桑比克', 'MZ'],
  '🇳🇨': ['New Caledonia', '新喀里多尼亚', 'NC'],
  '🇳🇫': ['Norfolk Island', '诺福克岛', 'NF'],
  '🇳🇵': ['Nepal', '尼泊尔', 'NP'],
  '🇳🇷': ['Nauru', '瑙鲁', 'NR'],
  '🇳🇺': ['Niue', '纽埃', 'NU'],
  '🇳🇿': ['New Zealand', '新西兰', 'NZ'],
  '🇴🇲': ['Oman', '阿曼', 'OM'],
  '🇵🇫': ['French Polynesia', '法属波利尼西亚', 'PF'],
  '🇵🇬': ['Papua New Guinea', '巴布亚新几内亚', 'PG'],
  '🇵🇰': ['Pakistan', '巴基斯坦', 'PK'],
  '🇵🇲': ['Saint Pierre and Miquelon', '圣皮埃尔和密克隆', 'PM'],
  '🇵🇳': ['Pitcairn Islands', '皮特凯恩群岛', 'PN'],
  '🇵🇷': ['Puerto Rico', '波多黎各', 'PR'],
  '🇵🇸': ['Palestine', '巴勒斯坦', 'PS'],
  '🇵🇼': ['Palau', '帕劳', 'PW'],
  '🇶🇦': ['Qatar', '卡塔尔', 'QA'],
  '🇷🇪': ['Réunion', '留尼汪', 'RE'],
  '🇷🇸': ['Serbia', '塞尔维亚', 'RS'],
  '🇸🇦': ['Saudi Arabia', '沙特阿拉伯', 'SA'],
  '🇸🇧': ['Solomon Islands', '所罗门群岛', 'SB'],
  '🇸🇨': ['Seychelles', '塞舌尔', 'SC'],
  '🇸🇭': ['Saint Helena', '圣赫勒拿', 'SH'],
  '🇸🇯': ['Svalbard and Jan Mayen', '斯瓦尔巴和扬马延', 'SJ'],
  '🇸🇲': ['San Marino', '圣马力诺', 'SM'],
  '🇸🇴': ['Somalia', '索马里', 'SO'],
  '🇸🇹': ['São Tomé and Príncipe', '圣多美和普林西比', 'ST'],
  '🇸🇽': ['Sint Maarten', '荷属圣马丁', 'SX'],
  '🇸🇾': ['Syria', '叙利亚', 'SY'],
  '🇸🇿': ['Eswatini', '斯威士兰', 'SZ'],
  '🇹🇦': ['Tristan da Cunha', '特里斯坦 - 达库尼亚', 'TA'],
  '🇹🇨': ['Turks and Caicos Islands', '特克斯和凯科斯群岛', 'TC'],
  '🇹🇫': ['French Southern Territories', '法属南部领地', 'TF'],
  '🇹🇯': ['Tajikistan', '塔吉克斯坦', 'TJ'],
  '🇹🇰': ['Tokelau', '托克劳', 'TK'],
  '🇹🇱': ['Timor-Leste', '东帝汶', 'TL'],
  '🇹🇲': ['Turkmenistan', '土库曼斯坦', 'TM'],
  '🇹🇴': ['Tonga', '汤加', 'TO'],
  '🇹🇻': ['Tuvalu', '图瓦卢', 'TV'],
  '🇺🇦': ['Ukraine', '乌克兰', 'UA'],
  '🇺🇲': ['U.S. Minor Outlying Islands', '美国本土外小岛屿', 'UM'],
  '🇺🇳': ['United Nations', '联合国', 'UN'],
  '🇺🇿': ['Uzbekistan', '乌兹别克斯坦', 'UZ'],
  '🇻🇦': ['Vatican City', '梵蒂冈', 'VA'],
  '🇻🇨': ['Saint Vincent and the Grenadines', '圣文森特和格林纳丁斯', 'VC'],
  '🇻🇬': ['British Virgin Islands', '英属维尔京群岛', 'VG'],
  '🇻🇮': ['U.S. Virgin Islands', '美属维尔京群岛', 'VI'],
  '🇻🇺': ['Vanuatu', '瓦努阿图', 'VU'],
  '🇼🇫': ['Wallis and Futuna', '瓦利斯和富图纳', 'WF'],
  '🇼🇸': ['Samoa', '萨摩亚', 'WS'],
  '🇽🇰': ['Kosovo', '科索沃', 'XK'],
  '🇾🇪': ['Yemen', '也门', 'YE'],
  '🇾🇹': ['Mayotte', '马约特', 'YT'],
}

export const emojiToRegionMap: Record<string, RegionInfo> = Object.fromEntries(
  Object.entries(COMPACT_REGION_DATA).map(([emoji, [en, zh, code, extra = []]]) => [
    emoji,
    {
      en,
      zh,
      code,
      aliases: [code.toLowerCase(), en.toLowerCase(), en.toLowerCase().replace(/\s+/g, ''), zh, ...extra],
    },
  ]),
)

const CODE_TO_REGION_MAP: Record<string, RegionInfo> = Object.fromEntries(
  Object.values(emojiToRegionMap).map(info => [info.code.toUpperCase(), info]),
)

const ISO_ALPHA2_REGEX = /^[A-Z]{2}$/

/**
 * 将由两个区域指示符符号组成的旗帜 emoji 转换为两字母 ISO 国家代码（如 🇸🇬 -> SG）
 */
export function getCountryCodeFromFlagEmoji(emoji: string): string | null {
  if (!emoji)
    return null
  const chars = Array.from(emoji.trim())
  if (chars.length !== 2)
    return null

  const codePoint1 = chars[0]?.codePointAt(0)
  const codePoint2 = chars[1]?.codePointAt(0)
  if (codePoint1 === undefined || codePoint2 === undefined)
    return null

  const REGIONAL_INDICATOR_START = 0x1F1E6
  const ASCII_ALPHA_START = 0x41

  if (
    codePoint1 >= REGIONAL_INDICATOR_START
    && codePoint1 <= 0x1F1FF
    && codePoint2 >= REGIONAL_INDICATOR_START
    && codePoint2 <= 0x1F1FF
  ) {
    const letter1 = String.fromCodePoint(codePoint1 - REGIONAL_INDICATOR_START + ASCII_ALPHA_START)
    const letter2 = String.fromCodePoint(codePoint2 - REGIONAL_INDICATOR_START + ASCII_ALPHA_START)
    return `${letter1}${letter2}`
  }

  return null
}

/**
 * 统一解析任意地区输入（emoji 国旗、两字母代码或特殊符号）为标准两字母代码，失败回退至 UN
 */
export function resolveRegionCode(region: string | null | undefined, fallback = 'UN'): string {
  if (!region)
    return fallback
  const trimmed = region.trim()
  if (!trimmed)
    return fallback

  const mapped = emojiToRegionMap[trimmed]
  if (mapped)
    return mapped.code

  const fromEmoji = getCountryCodeFromFlagEmoji(trimmed)
  if (fromEmoji)
    return fromEmoji

  const upper = trimmed.toUpperCase()
  if (ISO_ALPHA2_REGEX.test(upper))
    return upper

  if (trimmed === '🇺🇳' || trimmed === '🌐')
    return 'UN'

  return fallback
}

/**
 * 获取地区国旗图片路径（前后台统一）
 */
export function getFlagSrc(region: string | null | undefined): string {
  return `/assets/flags/${resolveRegionCode(region, 'UN')}.svg`
}

/**
 * 获取地区的显示名称
 */
export function getRegionDisplayName(region: string, language: 'en' | 'zh' = 'zh'): string {
  if (!region)
    return ''
  const trimmed = region.trim()
  const regionInfo = emojiToRegionMap[trimmed] || CODE_TO_REGION_MAP[trimmed.toUpperCase()]
  if (!regionInfo)
    return region

  return language === 'zh' ? regionInfo.zh : regionInfo.en
}

/**
 * 检查地区字段是否匹配搜索词
 */
export function isRegionMatch(regionEmoji: string, searchTerm: string): boolean {
  const lowerSearchTerm = searchTerm.toLowerCase().trim()
  if (!lowerSearchTerm)
    return true

  if (regionEmoji === searchTerm)
    return true

  const trimmed = regionEmoji.trim()
  const regionInfo = emojiToRegionMap[trimmed] || CODE_TO_REGION_MAP[trimmed.toUpperCase()]
  if (!regionInfo)
    return regionEmoji.toLowerCase().includes(lowerSearchTerm)

  if (regionInfo.en.toLowerCase().includes(lowerSearchTerm))
    return true

  if (regionInfo.zh.includes(lowerSearchTerm))
    return true

  return regionInfo.aliases.some(alias => alias.toLowerCase().includes(lowerSearchTerm))
}
