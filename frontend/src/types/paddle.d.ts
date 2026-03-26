interface PaddleEvent {
  name: string;
  data?: unknown;
}

interface PaddleCheckoutOpenOptions {
  transactionId: string;
}

interface PaddleInitializeOptions {
  token: string;
  eventCallback?: (event: PaddleEvent) => void;
}

declare const Paddle: {
  Environment: {
    set(environment: 'sandbox' | 'production'): void;
  };
  Initialize(options: PaddleInitializeOptions): void;
  Checkout: {
    open(options: PaddleCheckoutOpenOptions): void;
  };
};
