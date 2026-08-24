# System Design Write-Up (Point Form)

**Last-Mile Delivery Tracker**

Each point below runs 20 to 30 words. The main claim sits in the first 7 to 8 words. Required, concrete data comes before discursive justification. A `*` in the stack table marks a duplicate or scaling-only service, not part of the MVP build.

---

## 1. Rate calculation engine

- The engine runs eight fixed steps per order: zone detection, volumetric weight, chargeable weight, rate lookup, base charge, COD surcharge, fuel surcharge, GST.
- Zone detection happens first, from a pincode lookup, not a live calculation; this keeps the arithmetic that follows deterministic and independent of network calls.
- Volumetric weight equals length times width times height, divided by 5000, with dimensions in centimeters; this is the standard divisor across Indian couriers.
- Chargeable weight takes the higher of actual weight and volumetric weight; a lightweight, bulky package is billed on volume, not on the scale reading.
- Rate lookup pulls one active rate card per order type, B2B or B2C, plus one zone-pair row matching the pickup and drop zones.
- Base charge combines a first-slab price with a per-kg or per-slab rate for weight beyond that slab, both stored per zone pair, not hardcoded.
- COD surcharge applies only when payment type is COD; the schema stores it as a flat fee and a percentage, compared with a `GREATEST()` check.
- Fuel surcharge and GST apply last, as percentages of the running total; every percentage lives in `rate_cards`, so pricing changes need no redeploy.
- Every order stores a snapshot of the rate card and the full charge breakdown at creation time; a later price edit never alters a past order.

## 2. Zone detection

- Zone detection is a table lookup, not a coordinate calculation; each pincode maps to exactly one zone in the `zone_areas` table.
- This keeps order creation fast, since a single indexed query replaces any geospatial distance formula at the point where charge calculation begins.
- An admin owns zone boundaries directly, matching the real-world pattern where Indian couriers define zone tiers by distance and metro classification, not by formula.

## 3. Auto-assignment logic

- The assignment engine queries every agent in the pickup zone first, filtered to `availability = 'AVAILABLE'`, before any distance calculation runs.
- Haversine distance runs next, comparing each available agent's last known coordinates against the pickup address, and the nearest agent gets picked.
- A zone-widening fallback exists for the case with no available agent in the pickup zone; the search expands to adjacent zones before reaching admin.
- This is a greedy nearest-agent strategy, not a multi-order optimal-matching algorithm; the common case is one order needing one agent, not a batch.
- A full assignment-optimization algorithm, such as the Hungarian algorithm, adds complexity the MVP scope does not need; it stays a scaling note, not a build item.
- Every assignment writes one immutable row to `order_status_history`, whether automatic or manual, recording the actor and the timestamp of the decision.

## 4. Order status lifecycle and immutable tracking

- Nine status values define the lifecycle: `CREATED`, `CONFIRMED`, `ASSIGNED`, `PICKED_UP`, `IN_TRANSIT`, `OUT_FOR_DELIVERY`, `DELIVERED`, `FAILED`, `CANCELLED`, plus `RESCHEDULED` as a loop-back state.
- Every status change writes one row to `order_status_history`, an append-only table; a database trigger blocks `UPDATE` and `DELETE` on that table directly.
- Immutability holds at the database level, not by application convention alone; even a bug in the API code cannot silently rewrite delivery history.
- An admin can override any status directly, bypassing the linear flow; the override still writes a row with `actor_type = 'ADMIN'`, no exception granted.
- Role-based access control gates every write; the `users` table carries one `role` column, checked by backend middleware before a handler ever runs.

## 5. Failed delivery handling

- A failed delivery appends a `FAILED` row to history first; nothing about the earlier assignment or attempt gets deleted or overwritten at this step.
- A notification with a reschedule link follows immediately, sent to the customer, so the failure state never sits silent without a next action.
- A reschedule request writes to `reschedule_requests`, linking the previous agent to a new one, once the customer submits a new delivery date.
- The order's status becomes `RESCHEDULED`, then loops back into `ASSIGNED`, once auto-assignment finds a new agent for the newly requested date.
- Failed-delivery and RTO rates matter operationally beyond the schema itself; they are a large real-world cost lever, and history alone makes them measurable.

## 6. Notification integration

- Email uses Resend as the primary provider, offering 3,000 free emails per month and native support for React-based email templates ([sequenzy.com](https://www.sequenzy.com/blog/best-transactional-email-services)).
- Brevo is the marked alternate for email, at roughly 9,000 free emails per month, though it doubles as a marketing tool the MVP does not need ([ventureharbour.com](https://ventureharbour.com/transactional-email-service-best-mandrill-vs-sendgrid-vs-mailjet/)).
- SMS uses MSG91 as the primary provider, chosen for strong delivery rates in the Indian market and simple REST integration ([stateinfotech.com](https://stateinfotech.com/blogs/best-free-sms-otp-apis-for-developers)).
- Fast2SMS is the marked alternate for SMS, offering a ₹50 free trial credit against MSG91's no-free-trial policy ([fast2sms.com](https://www.fast2sms.com/help/bulk-sms-price-comparison/)).
- SendGrid was rejected outright for this project, since Twilio retired its permanent free plan in May 2025, leaving only a 60-day trial ([dreamlit.ai](https://dreamlit.ai/blog/best-sendgrid-alternatives)).
- Every send writes one row to the `notifications` table first, as a log rather than a queue, tracking channel, trigger status, and delivery outcome.

## 7. Stack summary table

A `*` marks a duplicate provider or a scaling-only addition; neither is required for the 36-hour MVP build.

| Layer | Service | Role |
|---|---|---|
| Frontend | Next.js (TypeScript) | Customer, agent, and admin UI, deployed as its own Railway service |
| Backend | Go + Fiber | API layer, rate engine, zone detector, assignment engine, notifier |
| Database | PostgreSQL (Railway) | All persistent data across nine normalized tables, plus append-only history |
| Auth | Clerk | Identity, session issuance, and role storage in JWT metadata |
| Email | Resend | Primary transactional email provider, 3,000 free sends per month |
| Email* | Brevo | Alternate provider, larger free tier, unused marketing features |
| SMS | MSG91 | Primary SMS provider, strong Indian delivery rates |
| SMS* | Fast2SMS | Alternate provider, ₹50 trial credit, India-only routing |
| Rate limiting | Postgres via `gofiber/storage` | Persists limiter counters across container restarts |
| Geospatial scaling* | Redis `GEOADD`/`GEOSEARCH` | Future replacement for per-request Haversine scans |
| Assignment scaling* | Hungarian algorithm | Future multi-order optimal-matching path, not greedy nearest-agent |
| Database scaling* | Neon (branching, scale-to-zero) | Future migration path once the MVP moves past demo scale |

## 8. Technology choice: Go and Fiber

- Throughput drives the backend-language decision first; Go handled 142,000 requests per second on JSON CRUD benchmarks, 3.7 times FastAPI's throughput ([acquaintsoft.com](https://acquaintsoft.com/blog/fastapi-vs-nodejs-vs-go-performance-benchmarks)).
- A second, independent source confirms the same gap from another angle; Go delivers 5 to 15 times Python's throughput specifically on CPU-bound work ([dev.to](https://dev.to/_d7eb1c1703182e3ce1782/python-vs-go-for-backend-development-in-2026-an-honest-comparison-3pno)).
- The rate engine is exactly that CPU-bound case; zone lookup, weight math, and surcharge stacking run as pure computation, not as I/O-bound waiting.
- Concurrency comes built into the language next, not layered on top of a single-threaded runtime; goroutines and channels are native, not an added keyword ([medium.com/@krunalkanojiya](https://medium.com/@krunalkanojiya/node-js-vs-go-in-2026-performance-concurrency-and-when-to-use-each-96392afe7430)).
- Status updates, location pings, and notification sends can each run as an independent goroutine, without extra coordination code or an external task queue.
- Deployment cost favors Go last, though it matters directly on a 36-hour timeline; the binary needs no runtime and starts instantly, unlike FastAPI or Node.js ([acquaintsoft.com](https://acquaintsoft.com/blog/fastapi-vs-nodejs-vs-go-performance-benchmarks)).
- A real trade-off exists here, not a free win; Go requires explicit error handling and manual request validation, work Pydantic handles automatically in FastAPI.
- The throughput and deployment gains outweigh that verbosity for this project's scope; the CPU-bound rate engine is the exact workload where Go's advantage holds.
