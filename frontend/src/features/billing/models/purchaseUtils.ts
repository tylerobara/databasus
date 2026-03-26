const BACKUPS_COMPRESSION_RATIO = 10;

function buildBackupSizeSteps(): number[] {
  const values: number[] = [];
  for (let i = 1; i <= 100; i++) values.push(i);
  for (let i = 110; i <= 200; i += 10) values.push(i);
  return values;
}

function buildStorageSizeSteps(): number[] {
  const values: number[] = [];
  for (let i = 20; i <= 100; i++) values.push(i);
  for (let i = 110; i <= 1000; i += 10) values.push(i);
  for (let i = 1100; i <= 5000; i += 100) values.push(i);
  for (let i = 6000; i <= 10000; i += 1000) values.push(i);
  return values;
}

const BACKUP_SIZE_STEPS = buildBackupSizeSteps();
const STORAGE_SIZE_STEPS = buildStorageSizeSteps();

const DB_SIZE_COMMANDS = [
  {
    label: 'PostgreSQL',
    code: `SELECT pg_size_pretty(pg_database_size(current_database()));`,
  },
  {
    label: 'MySQL / MariaDB',
    code: `SELECT table_schema AS 'Database',
  ROUND(SUM(data_length + index_length) / 1024 / 1024, 2) AS 'Size (MB)'
FROM information_schema.tables
GROUP BY table_schema;`,
  },
  {
    label: 'MongoDB',
    code: `db.stats(1024 * 1024)  // size in MB`,
  },
];

const POLL_INTERVAL_MS = 3000;
const POLL_TIMEOUT_MS = 2 * 60 * 1000;

function distributeGfs(total: number) {
  const daily = Math.min(7, total);
  const weekly = Math.min(4, Math.max(0, total - daily));
  const monthly = Math.min(12, Math.max(0, total - daily - weekly));
  const yearly = Math.max(0, total - daily - weekly - monthly);
  return { daily, weekly, monthly, yearly };
}

function formatSize(gb: number): string {
  if (gb >= 1000) {
    const tb = gb / 1000;
    return tb % 1 === 0 ? `${tb} TB` : `${tb.toFixed(1)} TB`;
  }
  return `${gb} GB`;
}

function sliderBackground(pos: number, max: number): React.CSSProperties {
  const pct = (pos / max) * 100;
  return {
    background: `linear-gradient(to right, #155dfc ${pct}%, #1f2937 ${pct}%)`,
  };
}

function findSliderPosForGb(gb: number): number {
  const idx = STORAGE_SIZE_STEPS.findIndex((s) => s >= gb);
  return idx === -1 ? STORAGE_SIZE_STEPS.length - 1 : idx;
}

export {
  BACKUPS_COMPRESSION_RATIO,
  BACKUP_SIZE_STEPS,
  STORAGE_SIZE_STEPS,
  DB_SIZE_COMMANDS,
  POLL_INTERVAL_MS,
  POLL_TIMEOUT_MS,
  distributeGfs,
  formatSize,
  sliderBackground,
  findSliderPosForGb,
};
