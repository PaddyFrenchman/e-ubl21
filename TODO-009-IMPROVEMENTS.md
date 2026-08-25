I checked the current `main` at **`91dc9e65a5159908a9d83ccee4bc4121af9f2a56`**. The implementation is a strong improvement, but I would **not mark TODO-009 fully closed yet**. There are two correctness issues worth fixing.

The basic state machine is good:

```text
accepting
   ↓
content durable
   ↓
enqueue_pending
   ↓
work published
   ↓
pending
   ↓
valid / invalid
```

Content-write failures go to `acceptance_failed` or `cleanup_pending`, and the reconciler repairs stale states every 30 seconds after a one-minute safety age.   The failure-injection tests also cover the major persistence boundaries.

The remaining problems are:

1. **🔴 Reconciliation is not permanently idempotent.** `PublishDocument()` uses the upload ID as `Nats-Msg-Id`, which is good, but the documents stream does **not** set `DuplicateWindow`.   JetStream deduplication is time-scoped, normally around two minutes by default, and a duplicate arriving outside that window is accepted as a new message. ([nats.io][1])

   So this sequence is possible:

```text
publish succeeds
↓
final KV "pending" write fails
↓
record remains enqueue_pending
↓
API crashes for > 2 minutes
↓
API restarts
↓
reconciler republishes same upload ID
↓
JetStream accepts it again
```

The code comment saying `PublishDocument is idempotent by upload ID` is therefore only **temporarily true**.

2. **🔴 Status transitions can regress because KV writes are unconditional.** `PutUploadRecord()` uses plain `kv.Put()`, with no revision/CAS protection.  The reconciler does:

```text
read enqueue_pending
publish
write pending
```

while the worker independently does:

```text
validate
write valid/invalid
```

The worker does not check/claim the record state before processing.  Therefore this race is possible:

```text
reconciler reads enqueue_pending
worker finishes → writes valid
reconciler finishes → writes pending
```

The final durable status becomes **pending**, even though validation already completed. Since the reconciler ignores `pending`, that record can remain stuck there.

3. **🟡 Reconciliation clears useful error information too early.** It sets `record.Error = ""` and persists `enqueue_pending` before attempting publication. If publication fails again, it increments an error counter but doesn't persist the new publication error into the record.  Recovery remains possible, but status polling loses the useful reason why the upload is stuck.

4. **🟡 The failure tests don't model ambiguous publish acknowledgement.** The injected `PublishDocument` either succeeds or fails cleanly. Real distributed failure can mean “JetStream stored the message, but the publisher never received the acknowledgement.” That's precisely the case where long-window duplicate handling matters. NATS specifically identifies this as a motivation for message-ID deduplication. ([nats.io][2])

### What I would change

The most important fix is to make upload-state transitions **monotonic with KV revisions/CAS**. For example:

```text
enqueue_pending revision 42
       ↓
CAS revision 42 → pending
```

If the worker has already changed revision 42 to `valid`, the reconciler's transition fails instead of overwriting it.

For publication idempotency, either configure a duplicate window compatible with the recovery horizon, or better, combine broker deduplication with an idempotent/claimed worker state so duplicate descriptors cannot cause duplicate processing even after the JetStream dedup window expires.

### Verdict

**TODO-009: 🟠 almost done, but I would reopen it.**

The acceptance/recovery architecture is good. The missing pieces are specifically **durable idempotence across long outages** and **CAS-protected state transitions**. Those are distributed-systems correctness issues, not cosmetic caveats.

There is also no CI status attached to this commit, so I can verify the committed code/tests, but not a GitHub-hosted test run.

[1]: https://nats.io/blog/jetstream-java-client-01-stream-create/?utm_source=chatgpt.com "JetStream Stream Creation with the NATS.io Java Library | NATS blog"
[2]: https://nats.io/blog/new-per-subject-discard-policy/?utm_source=chatgpt.com "Infinite message deduplication in JetStream | NATS blog"
