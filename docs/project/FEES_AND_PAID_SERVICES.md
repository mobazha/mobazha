# Mobazha Fees and Paid Services

Status: Public policy boundary; no fixed commercial rate is defined here
Last updated: 2026-06-29
Canonical language: English
Chinese translation: [FEES_AND_PAID_SERVICES_ZH.md](FEES_AND_PAID_SERVICES_ZH.md)

## Purpose

This document explains which costs are inherent to running Mobazha, which charges may come from optional services, and what a client must disclose before a user confirms a paid action.

It is a durable project-policy boundary, not a price list. Current Mobazha-operated service status and public pricing belong on [mobazha.org/fees](https://mobazha.org/fees). A transaction-specific quote, when implemented and shown, governs the amounts and transaction-specific terms only within this policy boundary.

## Core commitments

1. **Independent operation has no mandatory central Mobazha transaction fee.** Running Mobazha on infrastructure you control does not, by itself, create a fee owed to Mobazha for creating or completing an order.
2. **Operating the software is not costless.** Operators and users may still pay for servers, storage, network transactions, payment processors, exchange, taxes, support, or plugins.
3. **Optional services may be paid.** Mobazha or another provider may charge for hosting, managed transaction services, distribution, AI, storage, support, or other clearly identified services.
4. **Third-party costs remain separate.** A blockchain fee, payment-provider charge, plugin price, tax, or exchange cost must not be presented as a Mobazha platform fee unless Mobazha is the stated recipient.
5. **No blanket “free forever” claim.** Open-source availability and the no-mandatory-central-fee boundary do not mean that every deployment, plugin, network, or service is free.

## Cost and service categories

| Category | Typical payer | Recipient | Project rule |
|---|---|---|---|
| Mobazha software | Operator | None for license use, subject to the repository license | Self-hostable; no mandatory Mobazha order fee |
| Infrastructure | Operator | Hosting, storage, bandwidth, or other infrastructure provider | Selected and paid by the operator |
| Network or payment processing | Buyer, seller, or operator | Blockchain or payment provider | Quoted separately where known |
| Optional Mobazha service | The user selecting the service | Mobazha service operator | Requires clear pricing and consent |
| Third-party plugin or service | The user selecting it | Independent provider | Governed by that provider's disclosed terms |
| Seller-funded referral or distribution | Seller or market operator | Disclosed referrer, curator, or distributor | Optional, attributable, capped, and reversible on refund |

## Quote and disclosure requirements

Before a user confirms an action that creates a charge or reduces seller proceeds, the client should display, as applicable:

- the service provider and fee recipient;
- who pays the fee;
- the fee category and service being purchased;
- the fixed amount, percentage, minimum, and cap;
- the pricing and settlement assets;
- whether network and third-party costs are included;
- how cancellation, partial refund, refund, and dispute affect the fee;
- the quote expiry and rules version.

Fees must not be hidden in an unexplained exchange rate, spread, or aggregate deduction. A user-facing website or cached UI string is not a substitute for a transaction-specific quote.

The current release does not claim that a universal fee-quote protocol is already implemented. When such a contract is added, it must be versioned, fail closed when required information is missing, and be documented independently of any private service implementation.

## Managed transaction services

A provider may charge a transaction-related service fee only for an identified service, such as payment execution, delivery automation, evidence handling, dispute operations, or a defined risk commitment. The provider, payer, calculation, cap, and refund treatment must be visible before confirmation.

This is different from imposing a protocol tax on every Mobazha order. Self-hosted deployments must remain usable without enrolling in a Mobazha-operated managed transaction service.

## Referrals, distribution, and agents

Referral or distribution compensation must be:

- explicitly enabled and funded by the seller or market operator;
- tied to an attributable, settled transaction rather than registration or recruitment;
- subject to a total cap and a defined attribution window;
- reversed proportionally after refunds or fraud;
- disclosed when an agent or recommendation has a paid relationship.

Multi-level recruitment payments, unlimited downstream percentages, and undisclosed paid recommendations are outside the Mobazha project-policy boundary.

## Changes to this boundary

Any proposal to require a central Mobazha service or mandatory Mobazha fee for ordinary self-hosted orders requires:

1. a public ADR and contributor review;
2. an update to this document and the Mobazha release scope;
3. explicit client and API behavior;
4. migration and opt-out analysis;
5. license, consumer-protection, and operational review where applicable.

Service price changes that do not alter the independent-operation boundary should be published on the provider's pricing surface with an effective date. They do not require hard-coding a rate in this repository.

## Related documents

- [Mobazha Release Scope](RELEASE_SCOPE.md)
- [Payment Plugin Architecture](../plugins/PAYMENT_PLUGIN_ARCHITECTURE.md)
- [ADR-015: Payment Plugin Boundary](../adr/015-payment-plugin-boundary.md)
