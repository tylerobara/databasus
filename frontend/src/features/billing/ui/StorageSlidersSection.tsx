import styles from './PurchaseComponent.module.css';

import {
  BACKUP_SIZE_STEPS,
  DB_SIZE_COMMANDS,
  STORAGE_SIZE_STEPS,
  formatSize,
  sliderBackground,
} from '../models/purchaseUtils';
import { DbSizeCommands } from './DbSizeCommands';

interface Props {
  onStorageSliderChange: (pos: number) => void;
  backupSliderPos: number;
  onBackupSliderChange: (pos: number) => void;
  effectiveStoragePos: number;
  newStorageGb: number;
  singleBackupSizeGb: number;
  approximateDbSize: number;
}

export function StorageSlidersSection({
  onStorageSliderChange,
  backupSliderPos,
  onBackupSliderChange,
  effectiveStoragePos,
  newStorageGb,
  singleBackupSizeGb,
  approximateDbSize,
}: Props) {
  return (
    <div>
      {/* Storage size slider */}
      <div className="mb-4">
        <div className="mb-2 flex items-baseline">
          <label className="min-w-[100px] font-medium">Storage size</label>
          <span className="font-bold text-blue-500">{formatSize(newStorageGb)}</span>
        </div>

        <input
          type="range"
          className={styles.calcSlider}
          style={sliderBackground(effectiveStoragePos, STORAGE_SIZE_STEPS.length - 1)}
          min={0}
          max={STORAGE_SIZE_STEPS.length - 1}
          value={effectiveStoragePos}
          onChange={(e) => onStorageSliderChange(Number(e.target.value))}
        />

        <div className="mt-1.5 flex justify-between text-sm text-gray-500">
          <span>20 GB</span>
          <span>10 TB</span>
        </div>
      </div>

      {/* Single backup size slider */}
      <div className="mb-4">
        <div className="mb-1 flex items-baseline">
          <label className="min-w-[195px] font-medium">If your single backup size is</label>
          <span className="font-bold text-blue-500">{formatSize(singleBackupSizeGb)}</span>
        </div>

        <p className="mb-2 flex items-center text-gray-400">
          <span className="flex min-w-[185px] items-center gap-1">
            Then approximate DB size{' '}
            <span className="group relative inline-block">
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="-mt-0.5 inline cursor-help text-gray-500"
              >
                <circle cx="12" cy="12" r="10" />
                <path d="M12 16v-4M12 8h.01" />
              </svg>
              <span className="pointer-events-none absolute bottom-full left-1/2 mb-2 w-52 -translate-x-1/2 rounded-lg border border-[#ffffff20] bg-[#1f2937] px-3 py-2 text-gray-300 opacity-0 transition-opacity group-hover:opacity-100">
                Estimated with ~10x compression ratio typical for database backups. Can differ based
                on the database type, structure, and content.
              </span>
            </span>
          </span>
          <span className="font-medium text-gray-200">~{formatSize(approximateDbSize)}</span>
        </p>

        <DbSizeCommands commands={DB_SIZE_COMMANDS} />

        <input
          type="range"
          className={styles.calcSlider}
          style={sliderBackground(backupSliderPos, BACKUP_SIZE_STEPS.length - 1)}
          min={0}
          max={BACKUP_SIZE_STEPS.length - 1}
          value={backupSliderPos}
          onChange={(e) => onBackupSliderChange(Number(e.target.value))}
        />

        <div className="mt-1.5 flex justify-between text-sm text-gray-500">
          <span>1 GB</span>
          <span>200 GB</span>
        </div>
      </div>
    </div>
  );
}
