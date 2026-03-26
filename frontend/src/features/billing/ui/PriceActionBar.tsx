import { Button } from 'antd';

import { SubscriptionStatus } from '../../../entity/billing';

interface Props {
  monthlyPrice: number;
  currentPrice: number;
  isPurchaseFlow: boolean;
  isChangeFlow: boolean;
  isUpgrade: boolean;
  isDowngrade: boolean;
  isSameStorage: boolean;
  isSubmitting: boolean;
  subscriptionStatus: SubscriptionStatus;
  onPurchase: () => void;
  onChangeStorage: () => void;
}

export function PriceActionBar({
  monthlyPrice,
  currentPrice,
  isPurchaseFlow,
  isChangeFlow,
  isUpgrade,
  isDowngrade,
  isSameStorage,
  isSubmitting,
  subscriptionStatus,
  onPurchase,
  onChangeStorage,
}: Props) {
  return (
    <div className="mt-4 flex items-center gap-4 border-t border-[#ffffff20] pt-4">
      <div className="flex-1">
        <p className="text-2xl font-bold">
          ${monthlyPrice.toFixed(2)}
          <span className="text-base font-medium text-gray-400">/mo</span>
        </p>

        {isChangeFlow && !isSameStorage && (
          <p className="text-xs text-gray-400">Currently ${currentPrice.toFixed(2)}/mo</p>
        )}
      </div>

      <div className="flex flex-col items-end gap-1">
        {isPurchaseFlow && (
          <Button type="primary" size="large" loading={isSubmitting} onClick={onPurchase}>
            {subscriptionStatus === SubscriptionStatus.Canceled ? 'Re-subscribe' : 'Purchase'}
          </Button>
        )}

        {isChangeFlow && (
          <>
            <Button
              type="primary"
              size="large"
              loading={isSubmitting}
              disabled={!!isSameStorage}
              onClick={onChangeStorage}
            >
              {isUpgrade ? 'Upgrade' : isDowngrade ? 'Downgrade' : 'Change Storage'}
            </Button>

            {isDowngrade && (
              <p className="text-xs text-gray-500">
                Storage will be reduced from next billing cycle
              </p>
            )}
          </>
        )}
      </div>
    </div>
  );
}
