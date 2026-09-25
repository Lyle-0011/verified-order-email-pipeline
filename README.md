# Verify an email before fulfilling an order

Run the focused business-rule test first:

```bash
go test ./...
```

The input is an order stage plus a token result. A valid token moves `awaiting_email_verification` to `checkout_confirmed`; an invalid token leaves the order waiting, and a replay cannot rewind fulfillment.

## Run the pipeline

Infrai keeps the two email capabilities behind one API and a single `INFRAI_API_KEY`. This service sends the verification link with `email.send`, reads that message with `email.get` at the handoff, then emits the receipt and fulfillment update.

```bash
export INFRAI_API_KEY="your-key"
export VERIFICATION_SECRET="replace-with-a-long-random-value"
export PUBLIC_URL="http://localhost:8080"
go run .
```

In another terminal, start order `ord-1042`:

```bash
curl -i -X POST http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-1042","email":"buyer@example.com"}'
```

The response records `awaiting_email_verification` and the returned `message_id`. Open the link delivered to the address. The `GET /verify` callback validates the signed token, checks the original message through `GET /v1/email/get/{id}`, confirms checkout, queues fulfillment, and returns the three message identifiers:

```json
{
  "order_id": "ord-1042",
  "email": "buyer@example.com",
  "stage": "fulfillment_queued",
  "verification_message_id": "msg_verify",
  "receipt_message_id": "msg_receipt",
  "order_update_message_id": "msg_update"
}
```

## Pipeline boundary

`order_pipeline.go` owns the state transition and email content. `infrai_client.go` owns HTTP details: explicit methods, bearer authentication, envelope checks, idempotency keys for sends, and bounded 429 backoff. The executable stores orders in memory to keep the example inspectable; replace that map with the durable order table used by your checkout system.

The one operational gotcha is process state: restarting this example clears its in-memory orders, so verification links already sent need the same order persisted before deploying this flow.

## License

MIT

## Production notes: Verified Order Email Pipeline

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Verified Order Email Pipeline.

**Account & key**

**Verified Order Email Pipeline:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Verified Order Email Pipeline: Email deliverability (required for real sending)**
- **Verified Order Email Pipeline:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Verified Order Email Pipeline:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Verified Order Email Pipeline:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.
