import { Button, Modal, Spin } from 'antd';
import { useEffect, useState } from 'react';

import { usePurchaseFlow } from '../hooks/usePurchaseFlow';

import { CLOUD_PRICE_PER_GB } from '../../../constants';
import { SubscriptionStatus } from '../../../entity/billing';
import {
  BACKUPS_COMPRESSION_RATIO,
  BACKUP_SIZE_STEPS,
  STORAGE_SIZE_STEPS,
  distributeGfs,
  formatSize,
} from '../models/purchaseUtils';
import { BackupRetentionSection } from './BackupRetentionSection';
import { PriceActionBar } from './PriceActionBar';
import { StorageSlidersSection } from './StorageSlidersSection';

interface Props {
  databaseId: string;
  onSubscriptionChanged: () => void;
  onClose: () => void;
}

export function PurchaseComponent({ databaseId, onSubscriptionChanged, onClose }: Props) {
  const flow = usePurchaseFlow({ databaseId, onSubscriptionChanged, onClose });

  const [storageSliderPos, setStorageSliderPos] = useState(0);
  const [backupSliderPos, setBackupSliderPos] = useState(0);

  useEffect(() => {
    if (flow.initialSliderPos > 0) {
      setStorageSliderPos(flow.initialSliderPos);
    }
  }, [flow.initialSliderPos]);

  const singleBackupSizeGb = BACKUP_SIZE_STEPS[backupSliderPos];
  const minStoragePosIndex = STORAGE_SIZE_STEPS.findIndex((s) => s >= singleBackupSizeGb);
  const minStoragePos =
    minStoragePosIndex === -1 ? STORAGE_SIZE_STEPS.length - 1 : minStoragePosIndex;
  const effectiveStoragePos = Math.max(storageSliderPos, minStoragePos);
  const newStorageGb = STORAGE_SIZE_STEPS[effectiveStoragePos];
  const approximateDbSize = singleBackupSizeGb * BACKUPS_COMPRESSION_RATIO;
  const backupsFit = Math.floor(newStorageGb / singleBackupSizeGb);
  const gfs = distributeGfs(backupsFit);
  const monthlyPrice = newStorageGb * CLOUD_PRICE_PER_GB;

  const { subscription } = flow;

  const isPurchaseFlow =
    subscription &&
    (subscription.status === SubscriptionStatus.Trial ||
      subscription.status === SubscriptionStatus.Canceled ||
      subscription.status === SubscriptionStatus.Expired);

  const isChangeFlow =
    subscription &&
    (subscription.status === SubscriptionStatus.Active ||
      subscription.status === SubscriptionStatus.PastDue);

  const isUpgrade = isChangeFlow && newStorageGb > subscription.storageGb;
  const isDowngrade = isChangeFlow && newStorageGb < subscription.storageGb;
  const isSameStorage = isChangeFlow && newStorageGb === subscription.storageGb;
  const currentPrice = subscription ? subscription.storageGb * CLOUD_PRICE_PER_GB : 0;

  const modalTitle = isPurchaseFlow
    ? subscription.status === SubscriptionStatus.Canceled
      ? 'Re-subscribe'
      : 'Purchase subscription'
    : 'Change Storage';

  const isShowingForm =
    subscription &&
    !flow.isLoadingSubscription &&
    !flow.isWaitingForUpgrade &&
    !flow.isUpgradeTimedOut &&
    !flow.isWaitingForPayment &&
    !flow.isPaymentConfirmed &&
    !flow.isPaymentTimedOut &&
    !flow.isCheckoutOpen;

  return (
    <Modal
      title={modalTitle}
      open
      onCancel={onClose}
      footer={null}
      width={700}
      maskClosable={false}
    >
      {flow.isLoadingSubscription && (
        <div className="flex justify-center py-10">
          <Spin size="large" />
        </div>
      )}

      {flow.loadError && <div className="py-10 text-center text-red-500">{flow.loadError}</div>}

      {flow.isWaitingForPayment && (
        <div className="flex flex-col items-center gap-4 py-10">
          <Spin size="large" />
          <p className="text-gray-400">Confirming your payment...</p>
        </div>
      )}

      {flow.isPaymentConfirmed && (
        <div className="py-6 text-center">
          <p className="mb-1 text-lg font-semibold text-green-600 dark:text-green-400">
            Payment successful!
          </p>
          {flow.confirmedStorageGb !== undefined && (
            <p className="mb-4 text-gray-500 dark:text-gray-400">
              Your subscription is now active with {flow.confirmedStorageGb} GB of storage.
            </p>
          )}
          <Button type="primary" onClick={onClose}>
            OK
          </Button>
        </div>
      )}

      {flow.isPaymentTimedOut && (
        <div className="py-6 text-center">
          <p className="mb-4 text-yellow-500">
            Payment confirmation is taking longer than expected. Please reload the page to check the
            status.
          </p>
          <Button onClick={() => window.location.reload()}>Reload page</Button>
        </div>
      )}

      {flow.isUpgradeTimedOut && (
        <div className="py-6 text-center">
          <p className="mb-2 text-yellow-500">
            Upgrade is taking longer than expected, it will be applied shortly. Please reload the
            page
          </p>
          <Button onClick={onClose}>Close</Button>
        </div>
      )}

      {flow.isWaitingForUpgrade && !flow.isUpgradeTimedOut && (
        <div className="flex flex-col items-center gap-4 py-10">
          <Spin size="large" />
          <p className="text-gray-400">Waiting for storage upgrade confirmation...</p>
        </div>
      )}

      {isShowingForm && (
        <div>
          {isChangeFlow && subscription.pendingStorageGb !== undefined && (
            <div className="mb-4 rounded-lg border border-yellow-600/30 bg-yellow-900/20 px-4 py-3 text-sm text-yellow-400">
              Pending storage change to {formatSize(subscription.pendingStorageGb)} from next
              billing cycle
            </div>
          )}

          <div className="md:grid md:grid-cols-2 md:gap-6">
            <StorageSlidersSection
              onStorageSliderChange={setStorageSliderPos}
              backupSliderPos={backupSliderPos}
              onBackupSliderChange={setBackupSliderPos}
              effectiveStoragePos={effectiveStoragePos}
              newStorageGb={newStorageGb}
              singleBackupSizeGb={singleBackupSizeGb}
              approximateDbSize={approximateDbSize}
            />

            <BackupRetentionSection backupsFit={backupsFit} gfs={gfs} />
          </div>

          <PriceActionBar
            monthlyPrice={monthlyPrice}
            currentPrice={currentPrice}
            isPurchaseFlow={!!isPurchaseFlow}
            isChangeFlow={!!isChangeFlow}
            isUpgrade={!!isUpgrade}
            isDowngrade={!!isDowngrade}
            isSameStorage={!!isSameStorage}
            isSubmitting={flow.isSubmitting}
            subscriptionStatus={subscription.status}
            onPurchase={() => flow.handlePurchase(newStorageGb)}
            onChangeStorage={() => flow.handleStorageChange(newStorageGb)}
          />
        </div>
      )}
    </Modal>
  );
}
