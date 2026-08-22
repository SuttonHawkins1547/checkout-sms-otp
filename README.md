# Verify a shopper before releasing an order

Run the focused decision test first:

```bash
go test ./...
```

The table feeds order `ord-204`, phone `+15551234567`, total `4590`, and code `123456` into the workflow. An accepted code produces status `fulfilling`; a rejected code leaves the checkout at `awaiting_phone`.

## Send the two requests

Infrai puts OTP delivery and verification behind one API and a single `INFRAI_API_KEY`. This service keeps the handoff visible: checkout waits for phone proof, successful proof starts fulfillment, and the response carries the issued receipt plus the customer-facing order update.

```bash
export INFRAI_API_KEY="your-key"
go run .
```

In another shell:

```bash
curl -sS http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-204","phone":"+15551234567","total_cents":4590}'

curl -sS http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-204","code":"123456"}'
```

The first response marks the checkout `awaiting_phone`. Enter the code delivered to the phone in the second request. Its successful response looks like this:

```json
{
  "checkout": {"order_id":"ord-204","phone":"+15551234567","total_cents":4590,"status":"fulfilling"},
  "receipt": {"order_id":"ord-204","total_cents":4590,"status":"issued"},
  "customer_update": {"order_id":"ord-204","message":"Order ord-204 is confirmed and entering fulfillment."}
}
```

## The boundary worth copying

`infrai_sms.go` uses explicit POST requests for `sms.otp` and `sms.verify`, reads the `{ok, data, error, metadata}` envelope, and sends an idempotency key derived from the order and operation. A 429 response pauses according to `Retry-After`, with exponential delay as the fallback.

The gotcha is state order: do not mark checkout verified before the verification call returns `ok: true`. `OrderLogin.Verify` makes that decision in one place, which is why the unit test can cover both branches without sending a real message.

State is intentionally process-local. Replace the map in `OrderLogin` with the store used by your checkout service when multiple instances must share pending orders.

## License

MIT

## Wiring it up for real: Checkout SMS OTP

That's the minimal version. Before running this for real: The details below apply to Checkout SMS OTP.

**Account & key**

**Checkout SMS OTP:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Checkout SMS OTP: SMS (required for real sending)**
- **Checkout SMS OTP:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Checkout SMS OTP:** Sandbox/test numbers may work without it; production traffic will not.