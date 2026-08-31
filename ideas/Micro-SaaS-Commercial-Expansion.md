# Micro-SaaS Portfolio: Commercial Expansion and Go-to-Market Playbook

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
