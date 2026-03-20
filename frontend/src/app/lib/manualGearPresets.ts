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
    gear: 'Quiet',
    colorClass: 'text-emerald-500',
    borderClass: 'border-emerald-500/50',
    bgClass: 'bg-emerald-500/12',
    levels: [
      { level: 'Low', rpm: 1300 },
      { level: 'Mid', rpm: 1700 },
      { level: 'High', rpm: 1900 },
    ],
  },
  {
    gear: 'Standard',
    colorClass: 'text-blue-500',
    borderClass: 'border-blue-500/50',
    bgClass: 'bg-blue-500/12',
    levels: [
      { level: 'Low', rpm: 2100 },
      { level: 'Mid', rpm: 2310 },
      { level: 'High', rpm: 2760 },
    ],
  },
  {
    gear: 'Power',
    colorClass: 'text-purple-500',
    borderClass: 'border-purple-500/50',
    bgClass: 'bg-purple-500/12',
    levels: [
      { level: 'Low', rpm: 2800 },
      { level: 'Mid', rpm: 3000 },
      { level: 'High', rpm: 3300 },
    ],
  },
  {
    gear: 'Overclock',
    colorClass: 'text-orange-500',
    borderClass: 'border-orange-500/50',
    bgClass: 'bg-orange-500/12',
    levels: [
      { level: 'Low', rpm: 3500 },
      { level: 'Mid', rpm: 3700 },
      { level: 'High', rpm: 4000 },
    ],
  },
];

export const getManualGearHighLevelRpm = (gear?: string | null): number | undefined => {
  if (!gear) return undefined;
  const preset = MANUAL_GEAR_PRESETS.find((item) => item.gear === gear);
  return preset?.levels.find((level) => level.level === 'High')?.rpm;
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
