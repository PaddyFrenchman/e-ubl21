# Micro-SaaS Portfolio: Commercial Expansion and Go-to-Market Playbook

**Version:** 2.0  
**Date:** 21 August 2026

## Executive conclusion

The six ideas are not equally attractive commercially. The key questions are not only whether they can be built, but who buys, who controls the budget, what event creates urgency, how clearly the value can be explained, and how much support each customer will require.

| Rank | Product | First-sale difficulty | ARPU potential | Support burden | Overall |
|---|---|---:|---:|---:|---:|
| 1 | Agency Website Maintenance Evidence Platform | Low/Medium | €50–€250/mo | Low | ★★★★★ |
| 2 | Accessibility Regression Monitor | Medium | €29–€300/mo | Low/Medium | ★★★★★ |
| 3 | Domain & Email Security Monitor | Medium | €30–€300/mo | Very low | ★★★★☆ |
| 4 | EU E-Invoice Validation API | Medium | €50–€1,000+/mo | Medium | ★★★★☆ |
| 5 | DORA Register / ICT Evidence Manager | High | €1,500–€15,000+/yr | Medium/High | ★★★★☆ |
| 6 | Webhook Replay & Monitoring | High | €20–€300/mo | Low | ★★★☆☆ |

A useful strategic simplification is that the first three can share one crawler/scanner platform, while the e-invoice product is a separate validation engine and DORA is a structured compliance-data product.

---

# 1. Agency Website Maintenance Evidence Platform

## Positioning

Do not position this as another uptime monitor. The current market already has many agency-focused white-label monitoring products.

The stronger product is:

> **Automated proof that an agency's monthly maintenance retainer is delivering value.**

The core problem is not simply downtime. It is that agencies charge recurring maintenance fees, much of the work is invisible, and clients periodically ask what they are paying for.

## Who to sell to

Primary ICP:

- web agencies;
- WordPress agencies;
- Shopify agencies;
- Webflow agencies;
- freelance developers with maintenance retainers;
- managed WordPress hosts;
- small MSPs that also maintain websites.

Best size:

- 10–150 managed client sites;
- 2–30 employees;
- already selling maintenance or care plans.

Typical buyer:

- agency owner;
- managing director;
- technical director;
- head of web;
- operations director.

Avoid initially:

- one-site small businesses;
- major enterprise agencies;
- customers without recurring maintenance contracts.

## What pain are they buying relief from?

### Invisible work

The agency may charge €50, €100 or €200 per month. Most months nothing dramatic happens, which creates the perception that nothing was done.

### The client finds the failure first

Examples:

- contact form broken;
- checkout broken;
- SSL or DNS issue;
- broken links after a CMS update;
- marketing tag disappeared;
- accessibility regression;
- homepage slowed dramatically.

### Reporting consumes staff time

An employee may currently gather uptime, PageSpeed, plugin, security and support information manually every month.

## Marketing pitch

### Landing-page headline

> **Your clients pay for website maintenance. Show them what you did.**

### Subheading

> Continuously monitor every client site, catch regressions before the client does, and automatically send a branded maintenance report every month.

### Thirty-second pitch

> Your agency probably manages dozens of sites under monthly maintenance contracts. Most months the work is invisible until something breaks. Our platform continuously checks each client site for uptime, SSL, broken links, performance, accessibility, forms, DNS and other regressions, then automatically produces a branded monthly report under your agency's name. You get early warnings; your client gets visible proof that the maintenance fee is buying something.

## Best differentiation

The report should show:

- what was checked;
- what changed;
- what broke;
- what was fixed;
- what was prevented;
- what still needs attention.

A strong report is more valuable than a fancy dashboard.

Example:

```text
ACME AGENCY
Monthly Maintenance Report

Availability                         99.99%
SSL / domain                        Healthy
Broken links                        2 fixed
Accessibility regressions           3 found
Contact form synthetic test         Passed
Checkout synthetic test             Passed
Tracking tags                       No unexpected change

Changes this month
✓ CMS upgraded
✓ 3 plugins changed
✓ 2 broken links resolved
✓ DMARC still healthy
```

## High-value features

### Synthetic contact-form monitoring

Automatically submit a test form and verify that the workflow still succeeds.

Marketing value:

> We don't merely check that the homepage is online. We verify that customers can still contact you.

### Ecommerce journey monitoring

Verify:

```text
homepage → product → cart → checkout
```

without completing payment.

### Visual regression

Detect major layout failures, missing hero images, broken CSS, cookie overlays blocking pages, etc.

### Marketing-tag regression

Detect disappearance or change of Google Analytics, GTM, Meta Pixel and other nominated scripts.

This opens the product to marketing agencies as well as web developers.

## How to find prospects

Search for agencies already selling:

- website maintenance plans;
- care plans;
- WordPress support retainers;
- Shopify maintenance;
- managed hosting.

Sources:

- Clutch;
- DesignRush;
- LinkedIn;
- Google Maps;
- WordPress agency directories;
- Shopify Partners;
- Webflow Experts.

The strongest qualification signal is an agency website that explicitly advertises monthly maintenance plans.

## Cold outreach

> I noticed you offer monthly website maintenance plans. We built a platform that automatically monitors every client site and generates a branded monthly report showing uptime, regressions, broken links, accessibility and other maintenance checks. The purpose is simple: make the value of the maintenance retainer visible without your team writing reports manually. Send me one client domain and I can show you what the report would look like.

The best CTA is not “book a demo”. It is:

> **Send me one domain.**

## Pricing

- Solo: €29/month, 10 sites
- Agency: €69/month, 30 sites
- Agency Pro: €129/month, 75 sites
- Scale: €249/month, 200 sites
- Additional sites: roughly €1.50–€2.50/site/month

## Reseller economics

If an agency pays €129/month for 75 sites, its effective cost is €1.72/site. If it adds only €10/month to each maintenance plan, the gross recurring margin is substantial.

The pitch becomes:

> **Buy monitoring wholesale. Sell maintenance retail.**

## MVP

1. tenants and users;
2. clients and sites;
3. uptime;
4. TLS;
5. DNS/SPF/DMARC;
6. broken-link crawler;
7. accessibility scan;
8. Lighthouse/Chromium performance data;
9. screenshot;
10. change/regression history;
11. branded HTML report;
12. scheduled email;
13. billing.

## First 10 customers

Before building everything, manually contact 100 agencies. Offer a founder plan and generate a sample report from one of their domains.

The strongest validation signal is not “nice idea”. It is:

> “Can you put our logo on this and scan the other 40 sites?”

## Verdict

Best first business if positioned as **maintenance evidence and regression detection**, not generic monitoring.

---

# 2. Accessibility Regression Monitor

## Positioning

Do not sell automated “compliance certification”. Automated tooling cannot fully establish accessibility compliance.

Sell:

> **Continuous accessibility regression monitoring between human audits.**

## Who to sell to

Best customer groups:

1. web agencies maintaining ecommerce, transport, financial or public-sector sites;
2. ecommerce businesses;
3. accessibility consultancies;
4. SaaS product teams.

Typical buyers:

- agency owner;
- accessibility consultant;
- ecommerce digital lead;
- engineering manager;
- QA lead;
- compliance manager;
- product manager.

## Core pain

Accessibility is not static. A site can be professionally audited today and regress tomorrow because a developer or content editor deploys a new modal, form, colour scheme, third-party widget or inaccessible component.

The correct analogy is:

> **Production accessibility CI/CD.**

## Marketing pitch

Headline:

> **Know when your website becomes less accessible.**

Subheading:

> Continuously scan production websites for accessibility regressions and alert developers when a new issue appears.

Alternative:

> **Accessibility is not a one-time audit. Monitor what changes after the audit.**

Thirty-second pitch:

> Accessibility audits are snapshots. Websites keep changing. Our service continuously scans your production site, remembers the accessibility baseline and alerts your team only when a new regression appears. It does not pretend to replace a human WCAG audit. It prevents developers and content editors from quietly undoing the work after the audit.

## Best commercial wedge

Partner with accessibility consultants.

Consultants perform the expensive human audit. Your system maintains surveillance afterwards.

Pitch to consultants:

> Your audit identifies the problems. Our monitor tells your client when developers reintroduce them.

That creates recurring revenue attached to a one-off consulting engagement.

## Product

Core features:

- axe-core scan;
- crawl and page discovery;
- WCAG mapping;
- severity;
- first-seen and resolved timestamps;
- screenshots;
- DOM selector;
- baseline and change detection;
- weekly summary;
- Jira/GitHub/Linear integration later.

The key report is not total issues. It is:

```text
Last week: 37 known findings
This week: +2 new, -3 resolved, 36 total
```

This converts noise into an actionable engineering queue.

## Who to prospect

Good signals:

- companies publishing accessibility statements;
- ecommerce sites;
- booking/ticketing platforms;
- banks/payment firms;
- businesses advertising accessibility roles;
- agencies advertising WCAG/EAA work;
- accessibility consultants.

## Pricing

- Site: €19/month, 1 site
- Business: €49/month, 5 sites
- Agency: €129/month, 25 sites
- Consultant: €249/month, 75 sites and white-label

## Objection handling

“Automated scanners aren't enough.”

Correct response:

> Absolutely. We are not selling automated certification. We monitor regression between human audits.

This is a differentiator, not a weakness.

## Regulatory tailwind

The European Accessibility Act has applied since June 2025 to covered products and services, including e-commerce, banking services, certain transport services, electronic communications and others. Do not claim every website is automatically covered.

## Verdict

Potentially more defensible than generic website monitoring. Strong reseller channel through accessibility consultants.

---

# 3. Domain and Email Security Monitor

## Product concept

Continuously monitor infrastructure that companies configure once and then forget:

- domain expiry;
- nameservers;
- DNS;
- DNSSEC;
- SPF;
- DKIM;
- DMARC;
- MX;
- MTA-STS;
- TLS-RPT;
- certificate expiry;
- security/configuration drift.

## Who to sell to

Do not sell domain-by-domain to tiny businesses. Sell portfolios.

Primary ICP:

- MSPs;
- IT support companies;
- Microsoft 365 consultants;
- Google Workspace consultants;
- hosting companies;
- email deliverability consultants;
- cybersecurity consultancies;
- registrars/resellers.

Best portfolio size:

```text
25–1,000 customer domains
```

Typical buyers:

- MSP owner;
- head of managed services;
- technical director;
- Microsoft 365 consultant;
- security consultant.

## Core pain

Email authentication and DNS are invisible until they fail. A record changes, a vendor is added, a selector disappears or a domain nears expiry, and suddenly mail goes to junk or stops working.

The especially strong wedge is **configuration drift**.

Example:

```text
Yesterday:
SPF = Microsoft + Mailchimp

Today:
Mailchimp include disappeared

ALERT
```

or:

```text
DMARC changed from p=quarantine to p=none
```

## Marketing pitch

Headline:

> **Monitor every customer's email domain from one dashboard.**

Subheading:

> Catch SPF, DKIM, DMARC, DNS and certificate changes before they become deliverability tickets.

Thirty-second pitch:

> You manage Microsoft 365 or Google Workspace for dozens of customers, but critical email configuration lives in DNS outside those platforms. We continuously monitor each customer domain for SPF, DKIM, DMARC, MX, DNS and certificate drift. When something changes or becomes unsafe, your team gets an actionable alert. You can include a branded email-security health report in your managed-service package.

## Why now

Microsoft introduced SPF, DKIM and DMARC requirements in 2025 for domains sending more than 5,000 messages/day to Outlook.com consumer addresses. The broader market direction is toward stricter sender authentication.

## How to sell

Target companies advertising:

- Microsoft 365 support;
- Office 365 migration;
- managed IT;
- managed cybersecurity;
- Google Workspace support;
- email security.

Offer:

> Send me 10 customer domains. I will run a free portfolio scan.

Return a report such as:

```text
10 domains scanned
3 have no DMARC
1 SPF invalid
2 SPF near the lookup limit
1 certificate expires in 18 days
```

The scan demonstrates both the problem and a possible new managed-service upsell.

## Pricing

- MSP Starter: €49/month, 25 domains
- MSP: €99/month, 100 domains
- MSP Pro: €249/month, 500 domains
- Scale: custom

## Operational attractiveness

Very high. Most checks are DNS queries, TLS handshakes, HTTP and parsing. No headless browser is needed for the core product.

## Verdict

Probably the lowest-maintenance SaaS operationally. Slightly harder to explain than broken website monitoring, but attractive for MSP portfolios.

---

# 4. DORA Register / ICT Evidence Manager

## Narrow the scope

Do not build a universal “DORA + NIS2 + GDPR + ISO27001 compliance platform”. That becomes a large GRC system.

Start with one structured obligation:

> **DORA Register of Information for small and mid-sized regulated financial entities.**

## Why this is attractive

DORA became applicable on 17 January 2025. In-scope financial entities must maintain a comprehensive register of contractual arrangements with ICT third-party providers.

This is fundamentally a relational data problem involving:

- legal entities;
- ICT providers;
- contracts;
- ICT services;
- supported functions;
- critical/important functions;
- subcontractors;
- countries;
- identifiers;
- reporting relationships.

Spreadsheets become fragile quickly.

## Who to sell to

Do not start with major banks.

Best ICP:

- payment institutions;
- electronic money institutions;
- small investment firms;
- fintech firms;
- fund managers;
- smaller regulated entities with roughly 10–150 ICT providers.

Typical buyers:

- Chief Risk Officer;
- Head of Compliance;
- DORA Programme Manager;
- Operational Risk Manager;
- ICT Risk Manager;
- CISO;
- COO.

## Core pain

Many firms still maintain the register in interconnected Excel files and SharePoint folders. The data is relational, but the storage model is flat.

A single provider may have multiple contracts, services, legal entities, subcontractors and business functions.

The regulator expects a precise structured output, not an approximate vendor inventory.

## Marketing pitch

Headline:

> **Your DORA Register of Information, always regulator-ready.**

Subheading:

> Replace interconnected spreadsheets with a structured register of ICT providers, contracts, services, functions and subcontractors. Validate the data continuously and export the required reporting package when needed.

Alternative:

> **Stop rebuilding your DORA register in Excel.**

Thirty-second pitch:

> DORA requires financial entities to maintain a detailed Register of Information covering ICT third-party arrangements. The problem is that the data is relational but most teams maintain it in spreadsheets. Our platform stores providers, contracts, services, functions, entities and subcontractors as connected records, validates them continuously against the reporting model, tracks changes and generates a regulator-ready export. It is a register system, not a consulting project.

## Product wedge

The workflow should be:

```text
IMPORT → CLEAN → RELATE → VALIDATE → EXPORT → FREEZE SUBMISSION
```

Important features:

- import official templates/CSV;
- provider deduplication;
- identifier validation;
- provider/service/contract/function relationships;
- required-field checks;
- taxonomy validation;
- export of official reporting package;
- immutable submission snapshots;
- audit trail.

## Do not make AI central

AI can later extract data from contracts or suggest classifications, but the deterministic data model and validation rules must be authoritative.

## Best sales channel

Compliance consultants.

Why:

- they already have regulated clients;
- they already understand DORA;
- they can implement the register;
- you avoid becoming the customer's legal/regulatory adviser.

Pitch to consultant:

> Stop delivering a spreadsheet that becomes obsolete after your engagement. Give the client a live register and retain a recurring relationship.

## Direct sales

Use regulator and industry lists of authorised payment institutions, e-money institutions, investment firms and other in-scope entities as prospect databases.

This is a demo-led product, not initially a self-service €49 SaaS.

Suggested sales sequence:

```text
lead → qualification → sample import → show validation errors → annual contract
```

## Pricing

- Micro: €1,500/year, up to 20 ICT providers
- Small: €3,500/year, up to 75
- Professional: €6,000/year, up to 200 and multi-entity
- Group: €12,000+/year

Competitors already demonstrate that this market supports four-figure annual pricing.

## NIS2 expansion

Add later, not first.

NIS2 creates cybersecurity risk-management and incident-reporting obligations across many sectors, but national transposition matters. As of July 2026, the European Commission had referred Ireland, Spain, France and the Netherlands to the Court of Justice for incomplete transposition notification.

A safe product message is:

> Cybersecurity evidence mapped to NIS2 controls

not:

> Guaranteed NIS2 compliance.

## Operational burden

Expect requirements for:

- strong RBAC;
- audit logging;
- encryption;
- backups;
- EU hosting;
- DPA;
- penetration testing;
- SSO eventually.

## Verdict

High-value product, but not the lowest-attention first micro-SaaS. Better as a later-stage product with mature security and credibility.

---

# 5. EU E-Invoice Validation API

## Product concept

Answer one question extremely well:

> **Will this European e-invoice pass the required validation rules, and if not, exactly why?**

Relevant standards and layers include:

- UBL;
- CII;
- EN 16931;
- Peppol BIS;
- national CIUS profiles;
- XSD;
- Schematron;
- code lists.

## Why now

The EU ViDA package was adopted in March 2025 and is being rolled out progressively. From 1 July 2030, cross-border B2B transactions are scheduled to be subject to digital reporting requirements based on e-invoicing, while national e-invoicing programmes are already expanding across Europe.

Software vendors must prepare well before 2030.

## Who to sell to

Primary customers should be software companies that create or transform invoices:

### ERP vendors

Including vertical ERP for construction, logistics, hospitality, manufacturing and professional services.

### Accounting and invoicing SaaS

They need to generate and validate compliant e-invoices.

### Peppol service providers

Potential customers for validation, testing and debugging tooling.

### EDI/integration companies

They transform proprietary ERP formats into UBL/Peppol documents.

### Larger enterprises

Possible later, but slower procurement.

Typical buyers:

- CTO;
- Head of Engineering;
- Product Manager;
- Integration Lead.

## Core pain

The problem is not parsing XML.

It is understanding why an invoice that looks structurally valid is rejected by a Peppol or national profile.

Validation layers may be:

```text
well-formed XML
→ UBL XSD
→ EN16931
→ Peppol BIS
→ national CIUS
→ code-list rules
```

A document can pass one layer and fail the next.

## Marketing pitch

Headline:

> **One API to tell you whether a European e-invoice will pass.**

Subheading:

> Validate UBL and CII documents against EN 16931, Peppol BIS and supported national profiles. Receive structured errors with rule IDs, XPath locations and developer-readable explanations.

Alternative:

> **Stop debugging Peppol invoices by reading raw Schematron output.**

Thirty-second pitch:

> Your application can generate perfectly valid XML and still produce an invoice rejected by Peppol or a national profile. Our API detects the declared invoice profile, runs the relevant XSD, EN 16931, Peppol and country-specific rules, and returns structured JSON telling developers exactly which rule failed and where. You integrate one validation endpoint instead of maintaining a moving library of European rule sets.

## Best differentiation

A simple valid/invalid endpoint is commodity territory.

Developer experience is the moat.

Return structured data like:

```json
{
  "valid": false,
  "profile": "PEPPOL-BIS-3",
  "errors": [
    {
      "rule": "BR-CO-10",
      "severity": "fatal",
      "path": "/Invoice/...",
      "field": "invoice_total",
      "explanation": "...",
      "suggested_fix": "..."
    }
  ]
}
```

## Killer features

### Versioned validation

Allow customers to validate against:

- current production rules;
- a pinned historical version;
- a future/release-candidate ruleset.

This lets ERP vendors test future compatibility.

### Regression suites

A customer uploads 100 representative invoices. When a ruleset changes, run them again and report:

```text
98 still pass
2 newly fail
```

This turns the product into **e-invoice compatibility CI**.

### Private Docker deployment

Invoices contain sensitive commercial data. Offer an on-premises/private container edition.

Possible price:

```text
€3,000–€10,000/year
```

depending on SLA, volume and update rights.

## Sales and marketing

This should use developer-led growth.

Build:

- free public validator;
- excellent docs;
- CLI;
- GitHub examples;
- Docker demo image;
- SEO pages for exact Peppol rule IDs;
- articles explaining validation failures.

A developer searching for a specific failed rule is already a qualified prospect.

## Pricing

Example:

- Free: 100 validations/month
- Developer: €29/month
- Startup: €99/month
- Business: €299/month
- Enterprise: €750–€2,000+/month
- Private deployment: annual licence

## Scope discipline

Do not initially become a Peppol Access Point. Network certification, message transport and operational guarantees add a different class of complexity.

Validation is cleaner and more automatable.

## Verdict

Possibly the strongest long-term engineering/API product in the portfolio. More defensible than generic website monitoring, with stronger standards lock-in once integrated.

---

# 6. Webhook Replay and Monitoring SaaS

## Product concept

Capture inbound webhooks, persist them, forward them, inspect responses, retry and replay events.

Technically straightforward. Commercially crowded.

## Why generic positioning is weak

Developers already have many options:

- application logs;
- API gateways;
- queues;
- existing webhook services;
- custom retry logic.

Therefore avoid the generic message:

> Debug and replay webhooks.

## Better customer: integration agencies

Target companies managing many third-party integrations:

- ecommerce integration agencies;
- payment integrators;
- CRM consultants;
- SaaS integration consultancies;
- automation agencies.

Typical platforms:

- Stripe;
- Shopify;
- WooCommerce;
- HubSpot;
- Salesforce;
- shipping/payment systems.

Typical buyers:

- agency owner;
- CTO;
- senior developer;
- head of integration;
- DevOps lead.

## Core pain

A client says:

> “Order 10452 never reached our ERP.”

The developer needs to know:

- whether the webhook was sent;
- exact body and headers;
- endpoint response;
- signature state;
- whether it can be replayed.

Your service becomes a flight recorder.

## Marketing pitch

Headline:

> **Stop asking clients to reproduce failed integrations.**

Subheading:

> Capture every webhook your integrations receive, keep a searchable history, and replay any event after the bug is fixed.

For ecommerce agencies:

> **A black box recorder for Shopify, Stripe and WooCommerce integrations.**

## Product

- endpoint generation;
- encrypted payload storage;
- request body/headers;
- forwarding;
- retry policies;
- manual replay;
- searchable history;
- response logs;
- alerting;
- retention policies;
- PII redaction;
- team/RBAC.

## Stronger niche: private deployment

A self-hosted Docker version may be more interesting than another public webhook SaaS because many organisations do not want sensitive webhook payloads to transit a third party.

Sell:

- Docker Compose deployment;
- updates;
- SSO;
- support;
- commercial licence.

## Pricing

- Developer: €15/month
- Team: €49/month
- Business: €149/month
- Private deployment: €1,500–€5,000/year

## Verdict

The weakest of the six unless strongly verticalised or sold as a self-hosted/private tool.

---

# Cross-product sales map

| Product | Primary buyer | What they are really buying |
|---|---|---|
| Agency Maintenance Evidence | Agency owner / technical director | Client trust and retainer justification |
| Accessibility Monitor | Agency / consultant / ecommerce lead | Confidence that accessibility work is not regressing |
| Domain Security | MSP owner / managed-services lead | Fewer infrastructure and deliverability incidents |
| DORA Register | Compliance / risk / COO | Regulator readiness |
| E-Invoice API | CTO / engineering/product | Fewer rejected invoices and faster integration |
| Webhook Monitor | CTO / integration lead | Faster debugging |

---

# Purchase triggers

| Product | Trigger |
|---|---|
| Agency Maintenance | Client questions maintenance value; agency scales care plans |
| Accessibility | Audit, redesign, EAA project, accessibility incident |
| Domain Security | Deliverability incident; MSP creates security tier |
| DORA | Supervisory request, annual register update, audit |
| E-Invoice | Peppol integration, national mandate, ERP roadmap |
| Webhook | Production integration failure |

A strong product attaches itself to a trigger that creates urgency.

---

# Recommended sequencing

## Phase 1: shared scanner platform

Build one common engine supporting:

```text
scheduler
HTTP/TLS/DNS probes
crawler
Chromium
axe-core
screenshots
reports
multi-tenancy
billing
```

Launch first as:

> **Agency Maintenance Evidence**

## Phase 2

Reuse the crawler and launch:

> **Accessibility Regression Monitoring**

## Phase 3

Extract DNS/email functionality and launch:

> **Domain Security for MSPs**

The first three products share a large amount of infrastructure.

## Separate product line

Develop the e-invoice validator separately. It can share accounts, billing, API keys and tenancy, but its core validation engine is distinct.

## Later

Pursue DORA when the company itself has mature security, audit logging, backup procedures, EU hosting and credibility with regulated customers.

---

# Best first experiment

Before building a large SaaS, build one scanner that accepts a domain and generates a high-quality branded maintenance report.

Then contact 100 agencies that already sell maintenance plans.

Message:

> I noticed you offer website maintenance plans. I am testing a service that automatically produces a branded monthly evidence report for those plans. It checks site health, broken links, accessibility, performance and infrastructure changes. Send me one client domain and I will generate an example report under your agency's brand.

Measure:

```text
100 prospects
→ replies
→ domains submitted
→ requests for additional reports
→ willingness to pay
```

The strongest signal is not praise. It is a request to add many more sites.

---

# Revenue examples

Agency platform:

```text
100 customers × €79 = €7,900 MRR
300 customers × €79 = €23,700 MRR
```

Accessibility:

```text
100 customers × €100 average = €10,000 MRR
```

Domain monitor:

```text
100 MSPs × €125 average = €12,500 MRR
```

E-invoice API:

```text
50 software vendors × €300 average = €15,000 MRR
```

DORA:

```text
50 firms × €4,000/year = €200,000 ARR
```

DORA needs far fewer customers, but each sale is substantially harder.

---

# Final recommendation

There are two strongest strategic paths.

## Path A: fastest route to revenue

```text
Agency Maintenance Evidence
→ Accessibility
→ Domain/email security
```

Advantages:

- short sales cycles;
- small-business buyers;
- self-service possible;
- recurring subscriptions;
- one sale can bring many monitored sites;
- shared technical infrastructure.

## Path B: strongest specialist/API asset

```text
EU e-invoice validation
→ national profiles
→ regression testing
→ private Docker deployment
```

Advantages:

- deterministic standards engineering;
- technical buyers;
- regulatory tailwind;
- high switching cost after integration;
- potentially high-value annual licences.

If the goal is **first revenue quickly**, Path A is the better bet.

If the goal is **a more technically defensible specialist product**, the e-invoice validator may ultimately be the stronger asset.

---

# Current references

European Accessibility Act:  
https://commission.europa.eu/strategy-and-policy/policies/justice-and-fundamental-rights/disability/european-accessibility-act-eaa_en

Microsoft high-volume sender requirements:  
https://techcommunity.microsoft.com/blog/microsoftdefenderforoffice365blog/strengthening-email-ecosystem-outlook%E2%80%99s-new-requirements-for-high%E2%80%90volume-senders/4399730

DORA Register of Information:  
https://www.eba.europa.eu/activities/direct-supervision-and-oversight/digital-operational-resilience-act/preparation-dora-application

DORA ITS Register templates:  
https://www.eba.europa.eu/activities/single-rulebook/regulatory-activities/operational-resilience/implementing-technical-standards-establish-templates-register-information

NIS2:  
https://digital-strategy.ec.europa.eu/en/policies/nis2-directive

ViDA:  
https://taxation-customs.ec.europa.eu/taxation/vat/vat-digital-age-vida_en

Peppol certified providers:  
https://peppol.org/members/peppol-certified-service-providers/

Peppol BIS Billing documentation:  
https://docs.peppol.eu/poacc/billing/3.0/

Current agency-monitoring examples:  
https://pinglyra.com/  
https://tracefox.co/white-label-monitoring  
https://uptimeify.io/  
https://sitebrief.net/

Current DORA-product examples:  
https://www.treica.com/  
https://www.dora-guard.com/  
https://www.dokimos.eu/

Current e-invoice-validator examples:  
https://peppolvalidator.com/  
https://www.invoicexml.com/api/validate/ubl
