// One pipeline run, as data: the steps of the `ci` pipeline with when each starts and ends, and the
// log lines that follow from that. The page draws it twice (server side, as the finished run, so
// the page is whole without script; client side, as a loop) and TRUSS Desktop's run view is the
// model for both. Times are milliseconds from the start of one cycle.

export type RunStep = {
  id: string; // job/step, as TRUSS names it
  job: string;
  step: string;
  start: number;
  end: number;
  output: string[]; // what the step prints while it runs
};

export const STEPS: RunStep[] = [
  { id: 'lint/vet', job: 'lint', step: 'vet', start: 700, end: 2100, output: ['no issues found'] },
  {
    id: 'test/unit',
    job: 'test',
    step: 'unit',
    start: 700,
    end: 3500,
    output: ['ok  internal/engine  0.9s', 'ok  internal/executor  1.2s'],
  },
  { id: 'docs/build', job: 'docs', step: 'build', start: 700, end: 2900, output: ['9 pages built'] },
  { id: 'release/build', job: 'release', step: 'build', start: 3700, end: 5200, output: ['built bin/truss'] },
  { id: 'release/publish', job: 'release', step: 'publish', start: 5400, end: 6600, output: ['uploaded 6 archives'] },
];

export const SUMMARY_AT = 6700; // the run is done
export const CYCLE = 8600; // then the finished picture holds until the loop starts again

export const edges = [
  { from: 'lint/vet', to: 'release/build', d: 'M184 56H208V130H232' },
  { from: 'test/unit', to: 'release/build', d: 'M184 130H232' },
  { from: 'docs/build', to: 'release/build', d: 'M184 204H208V130H232' },
  { from: 'release/build', to: 'release/publish', d: 'M408 130H456' },
];

export const nodePos: Record<string, { x: number; y: number }> = {
  'lint/vet': { x: 8, y: 30 },
  'test/unit': { x: 8, y: 104 },
  'docs/build': { x: 8, y: 178 },
  'release/build': { x: 232, y: 104 },
  'release/publish': { x: 456, y: 104 },
};

export const stages = [
  { x: 8, label: 'stage 1 · 3 steps' },
  { x: 232, label: 'stage 2 · 1 step' },
  { x: 456, label: 'stage 3 · 1 step' },
];

export const duration = (s: RunStep): string => `${((s.end - s.start) / 1000).toFixed(1)}s`;

const START_CLOCK = 9 * 3600 + 14 * 60 + 2; // 09:14:02

export const stamp = (ms: number): string => {
  const total = START_CLOCK + Math.floor(ms / 1000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(Math.floor(total / 3600) % 24)}:${p(Math.floor(total / 60) % 60)}:${p(total % 60)}`;
};

export type LogLine = { at: number; id: string | null; text: string; kind: 'meta' | 'out' | 'ok' | 'sum' };

export const LOG: LogLine[] = (() => {
  const lines: LogLine[] = [];
  for (const s of STEPS) {
    lines.push({ at: s.start, id: s.id, text: 'started', kind: 'meta' });
    s.output.forEach((o, i) =>
      lines.push({ at: s.start + 350 + i * 420, id: s.id, text: `| ${o}`, kind: 'out' }),
    );
    lines.push({ at: s.end, id: s.id, text: `finished duration ${duration(s)} exit 0`, kind: 'ok' });
  }
  lines.push({
    at: SUMMARY_AT,
    id: null,
    text: `summary succeeded steps ${STEPS.length} duration ${((SUMMARY_AT - 700) / 1000).toFixed(1)}s`,
    kind: 'sum',
  });
  return lines.sort((a, b) => a.at - b.at);
})();
