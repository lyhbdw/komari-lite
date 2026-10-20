interface RetentionOption {
  label: string
  hours: number
}

/** Preserve preset order and append a custom retention limit when needed. */
export function buildRetentionOptions(presets: readonly RetentionOption[], hours: number): RetentionOption[] {
  const options = presets.filter(preset => hours >= preset.hours)
  const lastPreset = presets.at(-1)

  if ((lastPreset && hours > lastPreset.hours)
    || (hours > 1 && !presets.some(preset => preset.hours === hours))) {
    const label = hours % 24 === 0
      ? `${Math.floor(hours / 24)} 天`
      : `${hours} 小时`
    options.push({ label, hours })
  }

  return options
}
