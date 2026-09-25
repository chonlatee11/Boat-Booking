# Feature Research

**Domain:** Ferry/boat departure booking platform (Thailand island crossings), multi-pier/multi-operator marketplace
**Researched:** 2026-09-25
**Confidence:** MEDIUM (public web sources on competitor products and Thai market norms, cross-verified across multiple independent sources; no primary operator interviews)

## Feature Landscape

Sources surveyed: 12Go, Ferryhopper, Direct Ferries, Bookaway, Klook (ferry category), Boonsiri Ferry, Thai Ferry Tickets, plus PromptPay/LINE ecosystem norms and Thailand dual-pricing practice. Grouped below by the project's roles (Customer search & booking, Checkout & payment, Ticket & post-booking, Admin catalog & schedule, Admin bookings, Staff check-in, Notifications, Platform/i18n) per the downstream requirements consumer.

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes | M1 Scope |
|---------|--------------|------------|-------|----------|
| Search by origin pier, destination pier, date, pax | Every competitor's core entry point (12Go, Ferryhopper, Bookaway) — without it the product isn't a booking site | LOW | Already `Home/Search` in PROJECT.md §6.1 | **In M1** |
| Results list with departure time, price, seats-left badge | Standard comparison UX (Direct Ferries side-by-side fares, 12Go compare) | LOW | Card list per PROJECT.md UI direction | **In M1** |
| Adult/child passenger count selection | Universal on every ferry OTA (Direct Ferries explicit adult/child split) | LOW | Foreigner ticket type deferred to Growth per PROJECT.md | **In M1** (adult/child only) |
| Seat hold during checkout (no seat-map, just count) | Prevents booking two people into a sold-out departure mid-checkout; industry-standard even without seat maps | MEDIUM | 10-min hold, `SELECT...FOR UPDATE` — this project's core differentiator vs naive booking | **In M1** |
| Guest checkout (no forced signup) | 12Go/Bookaway/Ferryhopper all let you book without an account; forcing signup is a known conversion killer for tourist bookings made once | LOW-MEDIUM | Email/phone + OTP per PROJECT.md; account optional | **In M1** |
| Local payment method — PromptPay QR | PromptPay is the dominant Thai payment rail; online card usage is declining vs instant QR; omitting it makes checkout inaccessible to most Thai buyers, and it's also how many foreign tourists pay via banking apps once in-country | MEDIUM | Standard flow: select PromptPay → show QR → poll for bank confirmation | **In M1** |
| Card payment (international tourists) | Non-Thai tourists booking pre-arrival expect Visa/Mastercard; every reviewed competitor supports cards | MEDIUM | Same provider interface as PromptPay | **In M1** |
| Digital QR ticket (no printer needed) | Core value prop of this project and standard across 12Go/Ferryhopper/Klook — replaces paper vouchers | LOW-MEDIUM | QR token hash, save-image, large-and-readable-in-sun display | **In M1** |
| Email confirmation + resend | Universal — every OTA emails the ticket; "resend" is a standard self-service action | LOW | notification-service, idempotent | **In M1** |
| Booking lookup without login (ref + email) | Guest checkout requires a non-account way to retrieve a booking; Ferryhopper's "My Booking" page works this way | LOW | "My Bookings" screen in PROJECT.md | **In M1** |
| Transparent all-in pricing (no surprise fees at payment) | Direct Ferries and Ferryhopper both market "transparent fares, no hidden fees" as baseline trust; hidden fees are a top complaint pattern in ferry-OTA reviews | LOW | Show total (fare × pax) before checkout, not just per-ticket price | **In M1** |
| Clear cancellation policy shown before booking | No single Thai industry standard exists (policies vary 48h-full-refund vs 7-day-80%-tiers vs no-refund-once-confirmed), which makes *displaying* the specific policy pre-purchase table stakes — ambiguity is the #1 complaint source in ferry booking reviews | LOW | Route-level policy field already in domain model; **display in M1, execution (refund) deferred** | Display: **In M1**; refund execution: Deferred |
| Admin CRUD for piers/routes/boats/schedule | Every multi-operator marketplace needs self-service inventory management or it doesn't scale past one dev doing manual DB edits | MEDIUM | Already scoped in PROJECT.md Phase 1-2 | **In M1** |
| Capacity/availability accuracy (no overbooking) | The single most damaging failure mode in seat-based travel booking — an overbooked departure means a stranded tourist at a pier, which is worse than almost any UX flaw | HIGH | This is the project's stated Core Value; capacity race test in CI | **In M1** |
| Thai/English bilingual UI | Every Thai-market booking product (12Go, Klook Thailand, Thai Ferry Tickets) ships bilingual from day one because both Thai locals and foreign tourists are simultaneous user segments | MEDIUM | next-intl or similar, per-string not machine-translated | **In M1** |
| Mobile-first responsive UI | Booking-at-the-pier and booking-on-the-go are dominant use cases; every competitor is mobile-optimized | LOW-MEDIUM | Already in UI direction | **In M1** |
| Staff QR scan-to-check-in | Standard replacement for manual passenger-list ticking at every ferry pier product with a digital ticket | MEDIUM | Deferred — **Phase 5**, not M1 | Deferred |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes | M1 Scope |
|---------|-------------------|------------|-------|----------|
| Real-time seat availability (SSE) on search results | Most small Thai operator sites show static "call to check" availability; live badges ("ว่างอีก 12 ที่นั่ง") that update without refresh feel dramatically more trustworthy and reduce failed-booking frustration | MEDIUM-HIGH | Explicitly deferred to Polish phase; M1 ships correct-on-load availability via BFF aggregate query | Deferred (M1 has correct-but-not-live availability) |
| Split list+map layout (Zillow-style) | Competitors (12Go, Bookaway) are mostly form-and-list; a map-driven pier/route browsing experience is a genuine differentiator for a geography-heavy product (island crossings) | MEDIUM | Already the stated UI direction | **In M1** |
| Multi-pier/multi-operator single platform with per-operator admin scoping | Most small Thai speedboat operators run isolated booking pages (FerrySamui, Boonsiri, Thai Ferry Tickets are each single-operator); a real multi-operator marketplace with unified search is closer to 12Go/Bookaway's aggregator model but purpose-built for one geography — the actual competitive wedge | HIGH | Already the core architecture; `operator_id` scoping everywhere | **In M1** (foundation only; marketplace-scale discovery is later) |
| Friendly, localized micro-copy ("ว่างอัง 12 ที่นั่ง" not "Capacity: 12") | Competing Thai-market sites (12Go's Thai localization) are often stiffly translated; genuinely native-feeling copy builds trust with Thai users specifically | LOW | Already in UI direction | **In M1** |
| Pickup/transfer add-on bundling (hotel pickup + ferry) | Standard upsell pattern proven by every Phuket-Samui route seller (Lomprayah combo tickets, Easy Day Phuket/Samui, 12Go hotel pickup); real revenue lever once the core booking flow works | MEDIUM-HIGH | Requires a bookable "product" abstraction beyond single-route tickets — genuinely deferrable | Deferred (Growth) |
| LINE notifications (ticket + reminders) alongside email | LINE OA is the *primary* Thai digital touchpoint (not email) — Thai users expect near-instant confirmations there, and businesses that only use email under-serve the domestic segment | MEDIUM | Explicitly in PROJECT.md Growth phase; email-only is a deliberate M1 tradeoff, not an oversight | Deferred (Growth) |
| Foreigner ticket pricing tier | Dual pricing (Thai vs foreigner) is normalized and expected at Thai attractions/parks; adding it as a first-class `ticket_type` (already modeled) lets operators capture margin the way every comparable attraction does | LOW-MEDIUM (schema already supports it) | Domain model already has `foreigner` as a planned ticket_type; just not activated in M1 | Deferred (Growth) |
| Open-date / flexible-date tickets | Ferryhopper explicitly ships "convert to open ticket" self-service; valuable for tourists with uncertain itineraries but adds real inventory-management complexity (a ticket with no fixed departure competes with dated bookings for capacity) | HIGH | Not in current scope at all; flag as a possible post-Growth feature only if operators request it | Not scoped |
| Round-trip / combined outbound+return booking in one checkout | Direct Ferries and Ferryhopper both support this; reduces two separate bookings to one, higher perceived convenience | MEDIUM | Not currently modeled (Booking is per-departure); would need an order-level wrapper around two bookings | Not scoped (candidate for post-M1) |
| Group booking discounts / bulk pax pricing | Common ask for family/tour-group bookings; a pricing rule, not a workflow change, so cheap to add once ticket pricing exists | LOW-MEDIUM | No evidence this is universally expected (mixed across competitors) — treat as nice-to-have, not core | Not scoped |
| Promo codes | Growth-stage marketing lever once there's traffic to discount | LOW-MEDIUM | Already flagged in PROJECT.md Growth phase | Deferred (Growth) |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Seat map / assigned seating | Feels "more professional," mirrors airline UX | Ferries/speedboats rarely have fixed, sellable seat inventory identical trip-to-trip (boat swaps, weather-driven capacity changes); adds major UI/inventory complexity for a benefit few Thai ferry travelers actually value — none of the reviewed Thai-route sellers (FerrySamui, 12Go's ferry legs, Thai Ferry Tickets) offer seat selection | Ticket type + headcount only (already the project's explicit decision) |
| Native iOS/Android apps | "Feels more real," push notifications | Doubles the client surface for a booking flow that works fine as a PWA; every reviewed competitor (12Go, Ferryhopper, Bookaway) *also* has a web-first experience their apps just wrap; app-store review cycles slow iteration for a solo/small team | PWA with add-to-homescreen (already the project's explicit decision) |
| Full accounting suite / tax invoices (ใบกำกับภาษี) | Operators eventually want proper bookkeeping | Thai tax-invoice compliance (VAT format, sequential numbering, company details) is a deep compliance surface orthogonal to the booking flow; building it early blocks shipping the actual booking product | Simple receipt in v1; integrate a dedicated accounting tool or build compliance later once there's real revenue to justify it (already the project's explicit decision) |
| "Real-time everything" (live GPS boat tracking like Ferryhopper) | Looks impressive, "why not since we have the tech" | Requires boat-side GPS hardware/telemetry integration with zero current data source; solves a problem (where's my boat right now) that this project's core value prop — pre-departure booking/check-in — doesn't actually need | Accurate departure-time and status info; defer live tracking indefinitely unless operators supply GPS feeds |
| Unlimited free date-changes / fully flexible tickets by default | Feels customer-friendly, reduces support tickets superficially | Breaks the capacity/no-overbook guarantee that is this project's stated core value — a "flexible" ticket with no fixed departure is inventory nobody can plan against; also contradicts the tiered-refund-by-notice-window norm every reviewed Thai operator actually uses | Route-level cancellation policy (already modeled) + explicit "move to another departure" as an admin/support action, not a self-service default |
| Building a generic multi-operator billing/payout system in M1 | "We'll need it eventually for multi-operator" | Real payout/reconciliation logic (splits, fees, payout schedules, disputes) is a distinct hard problem from booking; premature build blocks shipping the booking core | Deferred to Growth per PROJECT.md; M1 proves the booking flow works for one or few operators manually reconciled |
| Deep discount/coupon engine with stacking rules, tiers, expiry logic | Marketing wants flexibility early | Full promo-rule engines (stacking, min-spend, per-user limits, A/B) are a rabbit hole with no revenue to justify them pre-launch | Single flat promo code field, deferred to Growth (already the project's plan) |

## Feature Dependencies

```
Search & Results (M1)
    └──requires──> Catalog (piers/routes/boats) (M1)
                       └──requires──> Operator/Pier admin CRUD (M1)

Departure Detail + Hold (M1)
    └──requires──> Schedule (departures generated from templates) (M1)
    └──requires──> Booking-service inventory projection (M1)

Checkout / Payment (M1)
    └──requires──> Hold (booking pending_payment) (M1)
    └──requires──> Payment provider (PromptPay + card) (M1)

Ticket issuance + Email (M1)
    └──requires──> PaymentSucceeded (M1)
    └──requires──> notification-service (M1)

Staff Check-in Scanner (Deferred, Phase 5)
    └──requires──> Ticket issuance (M1) [dependency already satisfied by M1]

Refund execution / Departure-cancel saga (Deferred, Phase 6)
    └──requires──> Cancellation policy display (M1) [policy must exist before refund logic can apply it]
    └──requires──> Payment provider Refund API (M1 interface, unused until Phase 6)

Real-time availability (SSE) (Deferred, Phase 7)
    └──enhances──> Search & Results (M1) [M1 works without it via polling/on-load fetch]

LINE notifications (Deferred, Growth)
    └──enhances──> Email notification (M1) [additive channel, not a replacement]

Foreigner ticket pricing (Deferred, Growth)
    └──enhances──> Ticket type model (M1 schema already anticipates adult/child/foreigner)

Pickup/transfer add-ons (Deferred, Growth)
    └──requires──> a bookable "product" concept beyond single-route ticket (not yet designed)

Round-trip combined checkout (Not scoped)
    └──requires──> Order-level wrapper spanning two Bookings (not in current domain model)
    └──conflicts with──> current 1-Booking-per-departure domain model; needs explicit design decision before adding

Open-date tickets (Not scoped)
    └──conflicts with──> No-overbooking guarantee (M1 core value) — an undated ticket competes with dated capacity; needs explicit inventory-holdback design before adding

Seat map (Anti-feature)
    └──conflicts with──> Headcount-only booking model (M1 explicit decision) — do not combine
```

### Dependency Notes

- **Real-time availability enhances Search, doesn't gate it:** M1 ships search/results with availability fetched live on page load (BFF aggregates catalog + `GetAvailability`) — this is "real-time enough" for a booking flow with a 10-minute hold; SSE only matters for *background* freshness while a user sits on the results page, which is a UX polish concern correctly deferred to Phase 7.
- **Cancellation policy display requires no new backend, refund execution does:** the route already carries a policy field in the M1 domain model, so *showing* it pre-purchase is nearly free. Actually *executing* refunds is a payment-provider-integration + saga problem, correctly scoped to Phase 6.
- **Open-date tickets conflict with the no-overbook guarantee:** this is the reason to explicitly not build it opportunistically later without a design pass — an "any date" ticket either needs its own reserved-but-unassigned capacity pool (complex) or silently oversells against dated bookings (breaks the core value prop). Flag for requirements definition to make an explicit call, not default into.
- **Round-trip conflicts with the current 1-booking-per-departure model:** don't let this get quietly bolted on inside M1 checkout — it needs an order/journey wrapper entity that doesn't exist yet.
- **Foreigner pricing is schema-ready but workflow-incomplete:** the ticket_type enum already anticipates `foreigner` per PROJECT.md; activating it in Growth is mostly a pricing-table + UI-selector change, not a new domain concept — cheap to add later, correctly deferred rather than built speculatively now.

## MVP Definition

### Launch With (v1 = Milestone 1, per PROJECT.md Phase 0-4)

- [ ] Search by pier/date/pax, results list with time/price/availability badge — the entire funnel starts here
- [ ] Adult/child passenger selection + 10-min seat hold, no overbooking — the project's stated Core Value
- [ ] Guest checkout via email/phone OTP — removes the #1 friction point competitors also avoid
- [ ] PromptPay QR + card payment — PromptPay is non-negotiable for the Thai market, card is non-negotiable for foreign tourists
- [ ] Digital QR ticket + email delivery + resend — the actual product being sold (no-queue boarding)
- [ ] My Bookings lookup (ref+email or login) — guest checkout requires this as its counterpart
- [ ] Cancellation policy *displayed* pre-purchase — table stakes for trust, cheap given the schema already models it
- [ ] Admin CRUD for piers/routes/boats/schedule templates/departures — without this, every route change is a manual DB edit and the platform can't onboard a second operator
- [ ] Thai/English i18n — both user segments (Thai locals + foreign tourists) are present from day one

### Add After Validation (v1.x — Phases 5-7 per PROJECT.md)

- [ ] Staff QR check-in scanner + passenger list — needed once real departures are running with real passengers boarding, but M1 can validate the booking funnel without it
- [ ] Refund execution + departure-cancellation saga — needed once real money is flowing and real departures start getting weather-cancelled
- [ ] SSE real-time availability — needed once traffic is high enough that stale-on-page availability causes failed checkouts
- [ ] Admin dashboard/reports/CSV export — needed once there's enough booking volume to need aggregate visibility

### Future Consideration (v2+ — Growth phase or later)

- [ ] LINE login/notifications — high value in Thailand but additive; email covers the M1 validation need
- [ ] Foreigner ticket pricing tier — real revenue lever, but defer until adult/child pricing proves the checkout flow works
- [ ] Pickup/transfer add-on bundling — real revenue lever, but needs a "product" abstraction not yet designed
- [ ] Promo codes — needs traffic to be worth building
- [ ] Multi-operator billing/payout — needs more than one paying operator to design against real requirements instead of guesses
- [ ] Round-trip combined checkout — needs an explicit domain-model decision (order wrapper) not yet made
- [ ] Open-date tickets — needs an explicit decision on how it interacts with the no-overbook guarantee before it's safe to build

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Search + results + hold (no overbook) | HIGH | HIGH | P1 |
| PromptPay + card checkout | HIGH | MEDIUM | P1 |
| QR ticket + email | HIGH | MEDIUM | P1 |
| Guest checkout (OTP) | HIGH | LOW | P1 |
| Admin catalog/schedule CRUD | HIGH | MEDIUM | P1 |
| Cancellation policy display | MEDIUM | LOW | P1 |
| Thai/English i18n | HIGH | MEDIUM | P1 |
| Staff check-in scanner | HIGH | MEDIUM | P2 |
| Refund + departure-cancel saga | HIGH | HIGH | P2 |
| SSE real-time availability | MEDIUM | MEDIUM-HIGH | P2 |
| LINE notifications | MEDIUM-HIGH (Thai segment) | MEDIUM | P3 |
| Foreigner pricing tier | MEDIUM | LOW | P3 |
| Pickup/transfer add-ons | MEDIUM-HIGH (revenue) | MEDIUM-HIGH | P3 |
| Round-trip combined checkout | MEDIUM | MEDIUM-HIGH | P3 |
| Open-date tickets | LOW-MEDIUM | HIGH | P3 |
| Seat map | LOW (for this product) | HIGH | Not planned (anti-feature) |
| Native apps | LOW (PWA covers it) | HIGH | Not planned (anti-feature) |

**Priority key:**
- P1: Must have for launch (Milestone 1)
- P2: Should have, add when possible (Milestones 5-7 per existing roadmap)
- P3: Nice to have, future consideration (Growth phase)

## Competitor Feature Analysis

| Feature | 12Go | Ferryhopper | Direct Ferries | This Project (M1) |
|---------|------|-------------|-----------------|--------------------|
| Multi-operator aggregation | Yes — broad Asia-wide, includes small local operators | Yes — 360+ operators, Mediterranean-focused | Yes — global, price-comparison focused | Yes, but geography-scoped (Thai islands) and operator-onboarded rather than scraped/aggregated |
| Passenger types | Adult/child implied via generic pax count | Adult/child/pet | Adult/child, foot vs vehicle, wheelchair/pet | Adult/child in M1; foreigner tier deferred to Growth |
| Payment methods | Cards, some local wallets | Cards, local methods vary by region | Cards | PromptPay QR + card (Thai-specific priority) |
| Seat hold / no-overbook guarantee | Not verifiable from public sources (opaque backend) | Not verifiable from public sources | Not verifiable from public sources | Explicit 10-min hold + `SELECT...FOR UPDATE`, verified via CI capacity race test — this is a stated, testable differentiator |
| Ticket delivery | Email | Email + app | Email | Email (QR) in M1; LINE deferred to Growth |
| Cancellation policy | Varies by underlying operator, opaque to buyer until booking | Self-service open-ticket conversion; policy varies by operator | Optional paid flexibility/insurance add-on | Route-level policy shown pre-purchase, tiered refund model (execution deferred to Phase 6) — more transparent upfront than typical aggregator UX |
| Real-time boat tracking | No | Yes (live map) | No | Not planned — no GPS data source, not core to value prop |
| Local admin self-service for small operators | No (12Go integrates operators on 12Go's terms) | No | No | Yes — first-class admin UI per operator/pier, a genuine gap versus pure aggregators |
| Open-date tickets | No | Yes | No | Not scoped — conflicts with no-overbook guarantee, needs explicit design |
| Pickup/transfer bundling | Yes (hotel pickup + ferry combos) | No (ferry-only) | No (ferry-only) | Deferred to Growth |

## Sources

- [12Go: Book Trains, Buses, Ferries, Transfers & Flights](https://12go.asia/en)
- [12Go Data Shows 49% Rise in Thailand Multimodal Travel Bookings](https://www.nationthailand.com/blogs/life/travel/40071433)
- [Ferryhopper — Ferry Trip Planning, Booking & Useful Info](https://www.ferryhopper.com/en/faq/reservation-booking-info)
- [Ferryhopper — How to Book your Tickets](https://www.ferryhopper.com/en/blog/how-to/how-to-book)
- [Ferryhopper — Ticket Changes, Cancellations & Refunds](https://www.ferryhopper.com/en/faq/ticket-changes-refunds-and-cancellations)
- [Direct Ferries — Booking FAQs](https://www.directferries.com/travel-info/booking-faqs)
- [Direct Ferries — Ferry Ticket Types](https://www.directferries.com/travel-info/ticket-types)
- [Bookaway — Discover how to get anywhere in Thailand](https://www.bookaway.com/routes/thailand)
- [Bookaway app listing](https://play.google.com/store/apps/details?id=com.bookaway.users&hl=en_US)
- [FerrySamui — Book Ferry Tickets](https://www.ferrysamui.com/)
- [Thai Ferry Tickets — Terms and Conditions](https://www.thaiferrytickets.com/en/terms-and-conditions)
- [Easy Day Phuket — Phuket to Koh Samui](https://www.easydayphuket.com/phuket-transfers/how-to-get-from-phuket-to-koh-samui/)
- [Boonsiri Ferry — Cancellation and Modification Policy](https://boonsiriferry.com/en/news/cancellation-and-modification-policy)
- [Klook — Can I cancel & refund my Thai ferry ticket booking?](https://www.klook.com/faq/category-107-question-9447/)
- [Thailand Ferry Booking condition page](https://thailandferrybooking.com/condition)
- [Tazapay — PromptPay for Business: Unlocking Higher Checkout Success in Thailand](https://tazapay.com/blog/promptpay-for-business-thailand-checkout-success)
- [Xendit — QR Payments in Thailand: Everything Your Business Needs to Know About PromptPay](https://www.xendit.co/en-th/blog/qr-payments-in-thailand-everything-your-business-needs-to-know-about-promptpay/)
- [8x8 CPaaS — Keep Customers Informed with Real-Time Updates on LINE](https://cpaas.8x8.com/en/blog/line-real-time-updates/)
- [LY Corporation — How "LINE for Business" in Thailand Drove 50% Sales Growth](https://www.lycorp.co.jp/en/story/20260106/lineth_forbusiness.html)
- [South China Morning Post — Thailand's dual pricing for foreign tourists in spotlight](https://www.scmp.com/magazines/post-magazine/travel/article/3094038/thailands-dual-pricing-foreign-tourists-spotlight)
- [Bangkok Post — Dual pricing as Thailand's tourism dilemma](https://www.bangkokpost.com/thailand/special-reports/3206548/dual-pricing-as-thailands-tourism-dilemma)
- [KohPlanner — Dual Pricing in Thailand](https://kohplanner.com/informative/dual-pricing-in-thailand/)

---
*Feature research for: Ferry/boat booking platform (Thailand island crossings)*
*Researched: 2026-09-25*
