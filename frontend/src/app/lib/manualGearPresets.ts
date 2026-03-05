export interface ManualGearPresetLevel {
  level: string;
  rpm: number;
}

export interface ManualGearPreset {
  gear: string;
  colorClass: string;
  borderClass: string;
  bgClass: string;
  levels: ManualGearPresetLevel[];
}

export const MANUAL_GEAR_PRESETS: ManualGearPreset[] = [
  {
    gear: '静音',
    colorClass: 'text-emerald-500',
    borderClass: 'border-emerald-500/50',
    bgClass: 'bg-emerald-500/12',
    levels: [
      { level: '低', rpm: 1300 },
      { level: '中', rpm: 1700 },
      { level: '高', rpm: 1900 },
    ],
  },
  {
    gear: '标准',
    colorClass: 'text-blue-500',
    borderClass: 'border-blue-500/50',
    bgClass: 'bg-blue-500/12',
    levels: [
      { level: '低', rpm: 2100 },
      { level: '中', rpm: 2310 },
      { level: '高', rpm: 2760 },
    ],
  },
  {
    gear: '强劲',
    colorClass: 'text-purple-500',
    borderClass: 'border-purple-500/50',
    bgClass: 'bg-purple-500/12',
    levels: [
      { level: '低', rpm: 2800 },
      { level: '中', rpm: 3000 },
      { level: '高', rpm: 3300 },
    ],
  },
  {
    gear: '超频',
    colorClass: 'text-orange-500',
    borderClass: 'border-orange-500/50',
    bgClass: 'bg-orange-500/12',
    levels: [
      { level: '低', rpm: 3500 },
      { level: '中', rpm: 3700 },
      { level: '高', rpm: 4000 },
    ],
  },
];

export const getManualGearHighLevelRpm = (gear?: string | null): number | undefined => {
  if (!gear) return undefined;
  const preset = MANUAL_GEAR_PRESETS.find((item) => item.gear === gear);
  return preset?.levels.find((level) => level.level === '高')?.rpm;
};

const MAX_GEAR_CODE_TO_RPM: Record<number, number> = {
  // Legacy max-gear codes observed in HID reports. 
  0x2: 2760,
  0x3: 2760,
  0x4: 3300,
  0x6: 4000,
  // Compatibility for firmware variants that use full gear codes.
  0xA: 2760,
  0xC: 3300,
  0xE: 4000,
};

export interface ReportedMaxRpmInfo {
  rpm?: number;
  codeHex?: string;
  source: 'gearSettings' | 'maxGearText' | 'unknown';
}

export const getReportedMaxRpm = (
  gearSettings?: number | null,
  maxGearText?: string | null,
): ReportedMaxRpmInfo => {
  if (typeof gearSettings === 'number') {
    const maxGearCode = (gearSettings >> 4) & 0x0f;
    const mapped = MAX_GEAR_CODE_TO_RPM[maxGearCode];
    if (mapped) {
      return { rpm: mapped, source: 'gearSettings' };
    }
    return { codeHex: `0x${maxGearCode.toString(16).toUpperCase()}`, source: 'gearSettings' };
  }

  const textMapped = getManualGearHighLevelRpm(maxGearText);
  if (textMapped) {
    return { rpm: textMapped, source: 'maxGearText' };
  }

  return { source: 'unknown' };
};
