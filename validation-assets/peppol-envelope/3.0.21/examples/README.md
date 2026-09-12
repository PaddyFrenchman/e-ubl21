# Envelope conformity corpus

These locally authored, minimal SBDH documents isolate the independently
reported envelope contract. `positive/` agrees with its embedded payload;
each file under `negative/` changes one envelope field and names the expected
rule in its filename. End-to-end tests additionally wrap a complete Peppol
Billing corpus Invoice, so payload-stage validity is tested separately.
