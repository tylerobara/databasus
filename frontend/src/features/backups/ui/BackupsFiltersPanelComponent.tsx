import { DatePicker, Select } from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';

import { BackupStatus, PgWalBackupType } from '../../../entity/backups';
import type { BackupsFilters } from '../../../entity/backups/api/backupsApi';

interface Props {
  filters: BackupsFilters;
  onFiltersChange: (filters: BackupsFilters) => void;
  isWalDatabase: boolean;
}

const statusOptions = [
  { label: 'In progress', value: BackupStatus.IN_PROGRESS },
  { label: 'Successful', value: BackupStatus.COMPLETED },
  { label: 'Failed', value: BackupStatus.FAILED },
  { label: 'Canceled', value: BackupStatus.CANCELED },
];

const pgWalBackupTypeOptions = [
  { label: 'Full backup', value: PgWalBackupType.PG_FULL_BACKUP },
  { label: 'WAL segment', value: PgWalBackupType.PG_WAL_SEGMENT },
];

export const BackupsFiltersPanelComponent = ({
  filters,
  onFiltersChange,
  isWalDatabase,
}: Props) => {
  const handleStatusChange = (statuses: string[]) => {
    onFiltersChange({ ...filters, statuses: statuses.length > 0 ? statuses : undefined });
  };

  const handleBeforeDateChange = (date: Dayjs | null) => {
    onFiltersChange({
      ...filters,
      beforeDate: date ? date.toISOString() : undefined,
    });
  };

  const handlePgWalBackupTypeChange = (value: string | undefined) => {
    onFiltersChange({ ...filters, pgWalBackupType: value });
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <span className="min-w-[90px] text-sm text-gray-500 dark:text-gray-400">Status</span>
        <Select
          mode="multiple"
          value={filters.statuses ?? []}
          onChange={handleStatusChange}
          options={statusOptions}
          placeholder="All statuses"
          size="small"
          variant="filled"
          className="w-[200px] [&_.ant-select-selector]:!rounded-md"
          allowClear
        />
      </div>

      <div className="flex items-center gap-2">
        <span className="min-w-[90px] text-sm text-gray-500 dark:text-gray-400">Before</span>
        <DatePicker
          value={filters.beforeDate ? dayjs(filters.beforeDate) : null}
          onChange={handleBeforeDateChange}
          size="small"
          variant="filled"
          className="w-[200px] !rounded-md"
          allowClear
        />
      </div>

      {isWalDatabase && (
        <div className="flex items-center gap-2">
          <span className="min-w-[90px] text-sm text-gray-500 dark:text-gray-400">Backup type</span>
          <Select
            value={filters.pgWalBackupType}
            onChange={handlePgWalBackupTypeChange}
            options={pgWalBackupTypeOptions}
            placeholder="All types"
            size="small"
            variant="filled"
            className="w-[200px] [&_.ant-select-selector]:!rounded-md"
            allowClear
          />
        </div>
      )}
    </div>
  );
};
