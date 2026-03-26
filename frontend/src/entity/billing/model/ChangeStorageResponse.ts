import { ChangeStorageApplyMode } from './ChangeStorageApplyMode';

export interface ChangeStorageResponse {
  applyMode: ChangeStorageApplyMode;
  currentGb: number;
  pendingGb?: number;
}
