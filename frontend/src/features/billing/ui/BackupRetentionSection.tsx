interface Gfs {
  daily: number;
  weekly: number;
  monthly: number;
  yearly: number;
}

interface Props {
  backupsFit: number;
  gfs: Gfs;
}

export function BackupRetentionSection({ backupsFit, gfs }: Props) {
  return (
    <div>
      <div className="space-y-1.5">
        <div className="rounded-lg border border-[#ffffff20] bg-[#1f2937]/50 px-3 py-2 text-center">
          <p className="text-gray-500">Total backups</p>
          <p className="text-lg font-bold text-gray-200">{backupsFit}</p>
        </div>

        <div className="my-1 flex items-center gap-3">
          <div className="h-px flex-1 bg-[#ffffff20]" />
          <span className="text-sm text-gray-500">or</span>
          <div className="h-px flex-1 bg-[#ffffff20]" />
        </div>

        <p className="mb-2 text-sm text-gray-400">
          Keeps recent backups frequently, older ones less often — broad time at the lowest cost. It
          is enough to keep the following amount of backups in GFS:
        </p>

        <div className="grid grid-cols-2 gap-1.5">
          {(
            [
              ['Daily', gfs.daily],
              ['Weekly', gfs.weekly],
              ['Monthly', gfs.monthly],
              ['Yearly', gfs.yearly],
            ] as const
          ).map(([label, value]) => (
            <div
              key={label}
              className="rounded-lg border border-[#ffffff20] bg-[#1f2937]/50 px-2 py-1.5 text-center"
            >
              <p className="text-xs text-gray-500">{label}</p>
              <p className="text-base font-bold text-gray-200">{value}</p>
            </div>
          ))}
        </div>
      </div>

      <p className="mt-2 text-sm text-gray-400">
        You can fine-tune retention values (change daily count, keep only monthly, keep N latest,
        etc.)
      </p>
    </div>
  );
}
