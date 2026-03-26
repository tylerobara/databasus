**Paddle hints:**

- **max_quantity on price:** Paddle limits `quantity` on a price to 100 by default. You need to explicitly set the range (`quantity: {minimum: 20, maximum: 10000}`) when creating a price via API or dashboard. Otherwise requests with quantity > 100 will return an error.
- **Full items list on update:** Unlike Stripe, Paddle requires sending **all** subscription items in `PATCH /subscriptions/{id}`, not just the changed ones. `proration_billing_mode` is also required. Without this you can accidentally remove a line item or get a 400.
- **Webhook events mapping:** Paddle uses `transaction.completed` instead of `payment.succeeded`, `transaction.payment_failed` instead of `payment.failed`, `adjustment.created` instead of `dispute.created`.
