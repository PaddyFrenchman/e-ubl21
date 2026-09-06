# Accepted Peppol release baseline

This directory freezes the first accepted normalized result set for Peppol BIS
Billing 3.0.21 and validation artefacts 1.3.16. It covers every fixture in the
tracked Peppol corpus, including XSD and EN short-circuit controls, generated
Profile 01/02 Invoice and CreditNote positives, rule-targeted negatives, and
the byte-pinned official examples.

The result set is not an automatically blessed implementation snapshot. The
metadata binds it to the corpus manifest, official Schematron source and
compiled XSLT digests, and the independent reviewed reference manifest. Tests
join reference cases to normalized results by fixture SHA-256 and require exact
Peppol outcome and official rule-ID equality. The normalized public contract
also freezes product-specific `EUBL21-PEPPOL-*` applicability failures for
explicit profile requests; those identifiers are intentionally kept separate
from official Schematron rule IDs. The mandatory reference gate additionally
recompiles the pinned Schematron and executes both the fresh and accepted
runtimes.

The issue #81 public split-pipeline gate must produce a byte-semantically
equivalent candidate with `regression.CompareResultSets`. A baseline change is
a standards-release review event: update the corpus, independent reference
evidence, metadata digests, normalized results, and upgrade rationale together.
Never replace this baseline merely because a candidate run differs.
