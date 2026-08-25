

The cleanest solution is to make the configured namespace itself canonical and reject ambiguous forms instead of trying to normalize arbitrary strings.

A good rule would be:

```text
^[a-z0-9]+(?:\.[a-z0-9]+)*$
```

So these are valid:

```text
eubl21
eubl21.dev
eubl21.test
eubl21.prod
customer1.prod
```

And these are rejected:

```text
eubl21--prod
eubl21_prod
.eubl21.prod
eubl21.prod.
eubl21..prod
EUBL21.Prod
```

Then the mapping stays deterministic:

```text
eubl21.prod
    │
    ├─ subjects
    │    eubl21.prod.documents.Invoice
    │    eubl21.prod.validation.results.<upload-id>
    │
    └─ NATS resource prefix
         EUBL21_PROD
```

I would also add tests specifically proving that invalid namespaces fail configuration rather than silently collapsing into another namespace.

Once that is done, **TODO-001 can be marked fully complete** and Phase A moves to **TODO-005**.
