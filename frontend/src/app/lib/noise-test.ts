// 风扇噪音测试：调用麦克风测量不同转速下的相对噪音水平，
// 生成转速-噪音档案用于优化学习模式，并检测共振转速区间。
//
// 测量原理：Web Audio AnalyserNode 取频谱，做 A 计权能量求和得到
// 相对声级（dBFS-A）。麦克风未经声学校准，绝对声压级不可知，
// 因此所有结果都以扫频中最安静的点为 0 dB 的「相对噪音」表示。
import { apiService } from '../services/api';
import { types } from '../../../wailsjs/go/models';

export interface NoiseProfilePoint {
  rpm: number;
  db: number;
}

export interface NoiseSweepSample {
  rpm: number;       // 下发的目标转速
  actualRpm: number; // 采样期间的实际转速
  db: number;        // 原始 A 计权相对声级 (dBFS-A)
}

export interface MicrophoneOption {
  deviceId: string;
  label: string;
}

/* ── A 计权 ── */

function aWeightDb(freq: number): number {
  const f2 = freq * freq;
  const ra =
    (12194 ** 2 * f2 * f2) /
    ((f2 + 20.6 ** 2) * Math.sqrt((f2 + 107.7 ** 2) * (f2 + 737.9 ** 2)) * (f2 + 12194 ** 2));
  return 20 * Math.log10(ra) + 2.0;
}

const MEASURE_FREQ_MIN = 40;
const MEASURE_FREQ_MAX = 16000;
const SILENCE_FLOOR_DB = -120;

function median(values: number[]): number {
  if (values.length === 0) return SILENCE_FLOOR_DB;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 0 ? (sorted[mid - 1] + sorted[mid]) / 2 : sorted[mid];
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) { reject(createAbortError()); return; }
    const timer = setTimeout(() => { cleanup(); resolve(); }, ms);
    const onAbort = () => { cleanup(); reject(createAbortError()); };
    const cleanup = () => { clearTimeout(timer); signal?.removeEventListener('abort', onAbort); };
    signal?.addEventListener('abort', onAbort);
  });
}

function createAbortError(): Error {
  return new DOMException('noise test aborted', 'AbortError');
}

export function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError';
}

/* ── 麦克风枚举与测量 ── */

// 枚举可用麦克风。需要先申请一次权限，否则设备列表里拿不到 label。
export async function listMicrophones(): Promise<MicrophoneOption[]> {
  if (!navigator.mediaDevices?.getUserMedia) {
    throw new Error('mediaDevices unavailable');
  }
  const probe = await navigator.mediaDevices.getUserMedia({ audio: true });
  try {
    const devices = await navigator.mediaDevices.enumerateDevices();
    return devices
      .filter((d) => d.kind === 'audioinput' && d.deviceId !== '')
      .map((d, i) => ({ deviceId: d.deviceId, label: d.label || `Microphone ${i + 1}` }));
  } finally {
    probe.getTracks().forEach((track) => track.stop());
  }
}

// NoiseMeter 持有一路麦克风输入并提供 A 计权声级测量。
// 测量用途必须关闭回声消除/降噪/自动增益，否则系统 DSP 会把风扇噪音当背景噪声滤掉。
export class NoiseMeter {
  private context: AudioContext | null = null;
  private stream: MediaStream | null = null;
  private analyser: AnalyserNode | null = null;
  private freqData: Float32Array | null = null;
  private binWeights: Float64Array | null = null;

  async open(deviceId?: string): Promise<void> {
    this.close();
    const constraints: MediaStreamConstraints = {
      audio: {
        ...(deviceId ? { deviceId: { exact: deviceId } } : {}),
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false,
      },
    };
    const stream = await navigator.mediaDevices.getUserMedia(constraints);
    const context = new AudioContext();
    const source = context.createMediaStreamSource(stream);
    const analyser = context.createAnalyser();
    analyser.fftSize = 4096;
    analyser.smoothingTimeConstant = 0.4;
    source.connect(analyser);

    const binHz = context.sampleRate / analyser.fftSize;
    const weights = new Float64Array(analyser.frequencyBinCount);
    for (let i = 0; i < weights.length; i++) {
      const f = i * binHz;
      weights[i] = f >= MEASURE_FREQ_MIN && f <= MEASURE_FREQ_MAX ? aWeightDb(f) : Number.NEGATIVE_INFINITY;
    }

    this.stream = stream;
    this.context = context;
    this.analyser = analyser;
    this.freqData = new Float32Array(analyser.frequencyBinCount);
    this.binWeights = weights;
    if (context.state === 'suspended') {
      await context.resume();
    }
  }

  get isOpen(): boolean {
    return this.analyser !== null;
  }

  // 单帧 A 计权相对声级 (dBFS-A)。
  readLevel(): number {
    if (!this.analyser || !this.freqData || !this.binWeights) return SILENCE_FLOOR_DB;
    this.analyser.getFloatFrequencyData(this.freqData as Float32Array<ArrayBuffer>);
    let energy = 0;
    for (let i = 1; i < this.freqData.length; i++) {
      const weight = this.binWeights[i];
      if (!Number.isFinite(weight)) continue;
      const v = this.freqData[i];
      if (!Number.isFinite(v)) continue;
      energy += 10 ** ((v + weight) / 10);
    }
    return energy > 0 ? 10 * Math.log10(energy) : SILENCE_FLOOR_DB;
  }

  // 在 durationMs 内按 intervalMs 周期采样，返回中位数声级与波动范围。
  // 中位数对说话声、键盘声等瞬时干扰更稳健。
  async sampleLevel(durationMs: number, signal?: AbortSignal, intervalMs = 100): Promise<{ db: number; rangeDb: number }> {
    const frames: number[] = [];
    const count = Math.max(3, Math.round(durationMs / intervalMs));
    for (let i = 0; i < count; i++) {
      await sleep(intervalMs, signal);
      frames.push(this.readLevel());
    }
    const valid = frames.filter((v) => v > SILENCE_FLOOR_DB);
    if (valid.length === 0) {
      return { db: SILENCE_FLOOR_DB, rangeDb: 0 };
    }
    return { db: median(valid), rangeDb: Math.max(...valid) - Math.min(...valid) };
  }

  close(): void {
    this.stream?.getTracks().forEach((track) => track.stop());
    void this.context?.close().catch(() => undefined);
    this.stream = null;
    this.context = null;
    this.analyser = null;
    this.freqData = null;
    this.binWeights = null;
  }
}

/* ── 风扇状态快照与恢复 ── */

export interface FanStateSnapshot {
  autoControl: boolean;
  customSpeedEnabled: boolean;
  customSpeedRPM: number;
  manualGear: string;
  manualLevel: string;
}

export function captureFanState(config: types.AppConfig): FanStateSnapshot {
  return {
    autoControl: !!config.autoControl,
    customSpeedEnabled: !!config.customSpeedEnabled,
    customSpeedRPM: config.customSpeedRPM || 0,
    manualGear: config.manualGear || '',
    manualLevel: config.manualLevel || '',
  };
}

// 测试结束后把风扇恢复到测试前的控制模式。
export async function restoreFanState(snapshot: FanStateSnapshot): Promise<void> {
  if (snapshot.customSpeedEnabled && snapshot.customSpeedRPM > 0) {
    await apiService.setCustomSpeed(true, snapshot.customSpeedRPM);
    return;
  }
  await apiService.setCustomSpeed(false, 0);
  if (snapshot.autoControl) {
    await apiService.setAutoControl(true);
    return;
  }
  if (snapshot.manualGear) {
    await apiService.setManualGear(snapshot.manualGear, snapshot.manualLevel);
  }
}

/* ── 扫频流程 ── */

export interface SweepOptions {
  minRpm: number;
  maxRpm: number;
  stepRpm: number;
  settleTimeoutMs?: number; // 等待转速到位的超时
  stabilizeMs?: number;     // 转速到位后的气流稳定时间
  sampleMs?: number;        // 每个转速点的采样时长
}

export interface SweepProgress {
  index: number;
  total: number;
  rpm: number;
  phase: 'settling' | 'sampling';
}

export function buildSweepSteps(minRpm: number, maxRpm: number, stepRpm: number): number[] {
  const steps: number[] = [];
  for (let rpm = minRpm; rpm < maxRpm; rpm += stepRpm) {
    steps.push(rpm);
  }
  steps.push(maxRpm);
  return steps;
}

async function waitForRpm(targetRpm: number, timeoutMs: number, signal?: AbortSignal): Promise<number> {
  const tolerance = Math.max(120, Math.round(targetRpm * 0.06));
  const deadline = Date.now() + timeoutMs;
  let lastRpm = -1;
  let hitCount = 0;
  let stallCount = 0;
  while (Date.now() < deadline) {
    await sleep(500, signal);
    let current = -1;
    try {
      const fanData = await apiService.getCurrentFanData();
      if (fanData && fanData.currentRpm > 0) current = fanData.currentRpm;
    } catch {
      /* 读取失败按未到位处理 */
    }
    if (current > 0) {
      hitCount = Math.abs(current - targetRpm) <= tolerance ? hitCount + 1 : 0;
      // 转速不再变化（如目标超出风扇能力上限）也视为已稳定
      stallCount = lastRpm > 0 && Math.abs(current - lastRpm) < 30 ? stallCount + 1 : 0;
      lastRpm = current;
      if (hitCount >= 2 || stallCount >= 4) {
        return current;
      }
    }
  }
  return lastRpm > 0 ? lastRpm : targetRpm;
}

// 执行 1000→4000 RPM 扫频测量。调用方负责事先 captureFanState、事后 restoreFanState。
export async function runNoiseSweep(
  meter: NoiseMeter,
  options: SweepOptions,
  onProgress: (progress: SweepProgress, samples: NoiseSweepSample[]) => void,
  signal?: AbortSignal,
): Promise<NoiseSweepSample[]> {
  const settleTimeoutMs = options.settleTimeoutMs ?? 20000;
  const stabilizeMs = options.stabilizeMs ?? 1500;
  const sampleMs = options.sampleMs ?? 3000;
  const steps = buildSweepSteps(options.minRpm, options.maxRpm, options.stepRpm);
  const samples: NoiseSweepSample[] = [];

  for (let i = 0; i < steps.length; i++) {
    if (signal?.aborted) throw createAbortError();
    const rpm = steps[i];
    onProgress({ index: i, total: steps.length, rpm, phase: 'settling' }, samples);
    await apiService.setCustomSpeed(true, rpm);
    const actualRpm = await waitForRpm(rpm, settleTimeoutMs, signal);
    await sleep(stabilizeMs, signal);

    onProgress({ index: i, total: steps.length, rpm, phase: 'sampling' }, samples);
    let { db, rangeDb } = await meter.sampleLevel(sampleMs, signal);
    // 波动过大说明有瞬时干扰（说话/碰撞声），重测一次取较稳的结果
    if (rangeDb > 6) {
      const retry = await meter.sampleLevel(sampleMs, signal);
      if (retry.rangeDb < rangeDb) {
        db = retry.db;
      }
    }
    samples.push({ rpm, actualRpm, db });
    onProgress({ index: i, total: steps.length, rpm, phase: 'sampling' }, samples);
  }

  return samples;
}

/* ── 结果分析 ── */

export interface ResonanceBand {
  startRpm: number;
  endRpm: number;
  peakRpm: number;
  prominenceDb: number;
}

export interface NoiseAnalysis {
  profile: NoiseProfilePoint[]; // 以最安静点为 0 dB 的相对档案（写入配置用）
  totalRiseDb: number;          // 最低到最高的总噪音上升
  kneeRpm: number | null;       // 噪音开始陡升的拐点转速
  resonance: ResonanceBand | null;
  lowConfidence: boolean;       // 总上升过小，测量区分度不足
}

const RESONANCE_PROMINENCE_DB = 2.5;
const LOW_CONFIDENCE_RISE_DB = 3;

export function analyzeNoiseSamples(samples: NoiseSweepSample[]): NoiseAnalysis {
  // 风扇达不到目标转速时（超出能力上限）以实际转速为准；正常到位时用目标转速保持网格整齐
  const resolved = samples.map((s) => {
    const tolerance = Math.max(120, s.rpm * 0.06);
    const rpm = s.actualRpm > 0 && Math.abs(s.actualRpm - s.rpm) > tolerance
      ? Math.round(s.actualRpm / 10) * 10
      : s.rpm;
    return { rpm, db: s.db };
  });
  resolved.sort((a, b) => a.rpm - b.rpm);

  // 合并相近转速点（间距 < 60 RPM 视为同一档，取平均声级）
  const merged: { rpm: number; db: number; count: number }[] = [];
  for (const s of resolved) {
    const last = merged[merged.length - 1];
    if (last && s.rpm - last.rpm < 60) {
      last.db = (last.db * last.count + s.db) / (last.count + 1);
      last.count += 1;
    } else {
      merged.push({ rpm: s.rpm, db: s.db, count: 1 });
    }
  }

  const minDb = Math.min(...merged.map((s) => s.db));
  const profile: NoiseProfilePoint[] = merged.map((s) => ({
    rpm: s.rpm,
    db: Math.round((s.db - minDb) * 10) / 10,
  }));

  const totalRiseDb = profile.length >= 2 ? Math.max(...profile.map((p) => p.db)) : 0;

  // 共振检测：内部点显著高于左右邻点连线 → 局部峰
  let resonance: ResonanceBand | null = null;
  for (let i = 1; i < profile.length - 1; i++) {
    const prev = profile[i - 1];
    const next = profile[i + 1];
    const span = next.rpm - prev.rpm;
    if (span <= 0) continue;
    const expected = prev.db + ((next.db - prev.db) * (profile[i].rpm - prev.rpm)) / span;
    const prominence = profile[i].db - expected;
    if (prominence >= RESONANCE_PROMINENCE_DB && (!resonance || prominence > resonance.prominenceDb)) {
      resonance = {
        startRpm: prev.rpm,
        endRpm: next.rpm,
        peakRpm: profile[i].rpm,
        prominenceDb: Math.round(prominence * 10) / 10,
      };
    }
  }

  // 拐点检测：段斜率超过平均斜率 1.5 倍、且其后仍有 ≥2dB 上升的第一个段起点
  let kneeRpm: number | null = null;
  if (profile.length >= 3 && totalRiseDb >= LOW_CONFIDENCE_RISE_DB) {
    const avgSlope = totalRiseDb / (profile[profile.length - 1].rpm - profile[0].rpm);
    const maxDb = Math.max(...profile.map((p) => p.db));
    for (let i = 0; i < profile.length - 1; i++) {
      const span = profile[i + 1].rpm - profile[i].rpm;
      if (span <= 0) continue;
      const slope = (profile[i + 1].db - profile[i].db) / span;
      if (slope > avgSlope * 1.5 && maxDb - profile[i].db >= 2) {
        kneeRpm = profile[i].rpm;
        break;
      }
    }
  }

  return {
    profile,
    totalRiseDb: Math.round(totalRiseDb * 10) / 10,
    kneeRpm,
    resonance,
    lowConfidence: totalRiseDb < LOW_CONFIDENCE_RISE_DB,
  };
}
