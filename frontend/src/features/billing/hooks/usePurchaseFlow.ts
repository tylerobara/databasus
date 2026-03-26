import { App } from 'antd';
import { useEffect, useRef, useState } from 'react';

import { ChangeStorageApplyMode, SubscriptionStatus, billingApi } from '../../../entity/billing';
import type { Subscription } from '../../../entity/billing';
import { POLL_INTERVAL_MS, POLL_TIMEOUT_MS, findSliderPosForGb } from '../models/purchaseUtils';

interface UsePurchaseFlowParams {
  databaseId: string;
  onSubscriptionChanged: () => void;
  onClose: () => void;
}

export function usePurchaseFlow({
  databaseId,
  onSubscriptionChanged,
  onClose,
}: UsePurchaseFlowParams) {
  const { message } = App.useApp();

  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [isLoadingSubscription, setIsLoadingSubscription] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isCheckoutOpen, setIsCheckoutOpen] = useState(false);
  const [isWaitingForPayment, setIsWaitingForPayment] = useState(false);
  const [isPaymentConfirmed, setIsPaymentConfirmed] = useState(false);
  const [confirmedStorageGb, setConfirmedStorageGb] = useState<number | undefined>();
  const [isPaymentTimedOut, setIsPaymentTimedOut] = useState(false);
  const [isWaitingForUpgrade, setIsWaitingForUpgrade] = useState(false);
  const [isUpgradeTimedOut, setIsUpgradeTimedOut] = useState(false);
  const [initialSliderPos, setInitialSliderPos] = useState(0);

  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const stopPolling = () => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
  };

  const loadSubscription = async () => {
    setIsLoadingSubscription(true);
    setLoadError(null);

    try {
      const sub = await billingApi.getSubscription(databaseId);
      setSubscription(sub);
      setInitialSliderPos(findSliderPosForGb(sub.storageGb));
    } catch {
      setLoadError('Failed to load subscription');
    } finally {
      setIsLoadingSubscription(false);
    }
  };

  const pollForPaymentConfirmation = () => {
    setIsWaitingForPayment(true);
    setIsPaymentTimedOut(false);

    pollingRef.current = setInterval(async () => {
      try {
        const sub = await billingApi.getSubscription(databaseId);

        if (
          sub.status !== SubscriptionStatus.Trial &&
          sub.status !== SubscriptionStatus.Expired &&
          sub.status !== SubscriptionStatus.Canceled
        ) {
          stopPolling();
          setIsWaitingForPayment(false);
          setIsPaymentConfirmed(true);
          setConfirmedStorageGb(sub.storageGb);
          onSubscriptionChanged();
        }
      } catch {
        // ignore polling errors, keep trying
      }
    }, POLL_INTERVAL_MS);

    timeoutRef.current = setTimeout(() => {
      stopPolling();
      setIsWaitingForPayment(false);
      setIsPaymentTimedOut(true);
    }, POLL_TIMEOUT_MS);
  };

  const pollForUpgradeConfirmation = (targetStorageGb: number) => {
    setIsWaitingForUpgrade(true);
    setIsUpgradeTimedOut(false);

    pollingRef.current = setInterval(async () => {
      try {
        const sub = await billingApi.getSubscription(databaseId);

        if (sub.storageGb === targetStorageGb && sub.pendingStorageGb === undefined) {
          stopPolling();
          setIsWaitingForUpgrade(false);
          onSubscriptionChanged();
          onClose();
        }
      } catch {
        // ignore polling errors, keep trying
      }
    }, POLL_INTERVAL_MS);

    timeoutRef.current = setTimeout(() => {
      stopPolling();
      setIsWaitingForUpgrade(false);
      setIsUpgradeTimedOut(true);
    }, POLL_TIMEOUT_MS);
  };

  const handlePurchase = async (storageGb: number) => {
    setIsSubmitting(true);

    try {
      const result = await billingApi.createSubscription(databaseId, storageGb);

      setIsCheckoutOpen(true);
      setIsSubmitting(false);

      Paddle.Checkout.open({
        transactionId: result.paddleTransactionId,
      });
    } catch {
      message.error('Failed to create subscription');
      setIsSubmitting(false);
    }
  };

  const handleStorageChange = async (storageGb: number) => {
    if (!subscription) return;

    setIsSubmitting(true);

    try {
      const result = await billingApi.changeStorage(databaseId, storageGb);

      if (result.applyMode === ChangeStorageApplyMode.Immediate) {
        setIsSubmitting(false);
        pollForUpgradeConfirmation(storageGb);
      } else {
        setIsSubmitting(false);
        onSubscriptionChanged();
        onClose();
      }
    } catch {
      message.error('Failed to change storage');
      setIsSubmitting(false);
    }
  };

  useEffect(() => {
    loadSubscription();

    return () => stopPolling();
  }, [databaseId]);

  useEffect(() => {
    const handlePaddleEvent = (e: Event) => {
      const event = (e as CustomEvent<PaddleEvent>).detail;

      if (event.name === 'checkout.completed') {
        setIsCheckoutOpen(false);
        pollForPaymentConfirmation();
      } else if (event.name === 'checkout.closed') {
        setIsCheckoutOpen(false);
      }
    };

    window.addEventListener('paddle-event', handlePaddleEvent);

    return () => window.removeEventListener('paddle-event', handlePaddleEvent);
  }, [databaseId]);

  return {
    subscription,
    isLoadingSubscription,
    loadError,
    isSubmitting,
    isCheckoutOpen,
    isWaitingForPayment,
    isPaymentConfirmed,
    confirmedStorageGb,
    isPaymentTimedOut,
    isWaitingForUpgrade,
    isUpgradeTimedOut,
    initialSliderPos,
    handlePurchase,
    handleStorageChange,
  };
}
