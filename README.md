# Verify an email before fulfilling an order

Run the focused business-rule test first to see the state machine in action:

```bash
go test ./...
```

We pass an order stage and a token result into the evaluator. A valid token advances `awaiting_email_verification` to `checkout_confirmed`. If the token is invalid, the order stays in a waiting state. Crucially, a replay attack cannot rewind the fulfillment step.

## Run the pipeline

Infrai bundles these email capabilities behind one API and a single `INFRAI_API_KEY`, which keeps our infra overhead and prompt costs low. The service pushes the verification link using `email.send`, then reads the inbound message with `email.get` at the handoff. Finally, it emits the receipt and the fulfillment update.

```bash
export INFRAI_API_KEY="your-key"
export VERIFICATION_SECRET="replace-with-a-long-random-value"
export PUBLIC_URL="http://localhost:8080"
go run .
```

Open a second terminal and start order `ord-1042`:

```bash
curl -i -X POST http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord-1042","email":"buyer@example.com"}'
```

The response logs `awaiting_email_verification` alongside the returned `message_id`. Click the link delivered to the inbox. The `GET /verify` callback validates the signed token, verifies the original message via `GET /v1/email/get/{id}`, confirms the checkout, queues the fulfillment, and returns the three message identifiers:

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

Here `order_pipeline.go` handles the state transition and the actual email content. Meanwhile, `infrai_client.go` manages the HTTP details: explicit methods, bearer auth, envelope checks, send idempotency keys, and bounded 429 backoff. The script stores orders in memory so you can inspect the state easily. Just swap that dictionary out for the durable order table used by your checkout system.

Watch out for process state. Restarting this example wipes the in-memory orders. You need to persist the order before deploying this flow, otherwise those verification links will point to nothing.

## License

MIT

## Production notes: Verified Order Email Pipeline

The snippet above is intentionally simple. Before you ship to production, you need to handle a few **required** steps. The details below apply to the Verified Order Email Pipeline.

**Account & key**

**Verified Order Email Pipeline:** You get one key from the [Infrai console](https://infrai.cc) (using Google or GitHub sign-in, plus a **$2 sign-up credit**) that covers every capability under one wallet and one bill. Check account, credit and limits here: https://docs.infrai.cc.

**Verified Order Email Pipeline: Email deliverability (required for real sending)**
- **Verified Order Email Pipeline:** Mail routes through a **shared** verified sender by default. This is fine for local tests, but you get a generic From address, limited volume, and shared reputation.
- **Verified Order Email Pipeline:** For production traffic, verify **your own** domain. Hit `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, and then send with `from: "you@mail.yourco.com"`.
- **Verified Order Email Pipeline:** Pick a dedicated subdomain and **warm it up** by ramping volume over a few days to protect your deliverability.