.PHONY: test-en16931 test-en16931-compliance test-en16931-image test-peppol-schematron test-peppol-positive test-peppol-negative test-validation-chaos benchmark-en16931 benchmark-stages benchmark-stages-cgroup regression-correctness regression-equivalence regression-minikube regression-minikube-clean

# Run positive and targeted-negative EN 16931 corpora, plus the XSD
# short-circuit control, with runtime skipping disabled.
test-en16931:
	./devsetup/test-en16931.sh

# Explicit release-gate name. test-en16931 includes the same final public API,
# crash/restart, operational replay, and duplicate-replay scenarios.
test-en16931-compliance: test-en16931

# Build the production image, then prove its checksum-verified Saxon smoke
# transform succeeds with networking disabled. Override IMAGE if required.
test-en16931-image:
	./devsetup/test-en16931-image.sh "$(or $(IMAGE),e-ubl21:en16931-check)"

# Recompile the pinned Peppol Schematron twice, compare it with the accepted
# executable XSLT, and run the offline SVRL smoke transform.
test-peppol-schematron:
	./devsetup/test-peppol-schematron.sh

# Run every positive Peppol corpus fixture through UBL XSD, the pinned EN
# ruleset, and the pinned Peppol ruleset. Runtime dependencies are mandatory.
test-peppol-positive:
	./devsetup/test-peppol-positive.sh

# Prove every targeted negative is XSD/EN valid before it reaches Peppol and
# emits exactly its catalogued official rule evidence. Earlier-stage controls
# prove XSD-invalid and EN-invalid documents stop before later runtimes.
test-peppol-negative:
	./devsetup/test-peppol-negative.sh

# Run the deterministic JetStream chaos/crash/replay integration gate. The
# scenario-to-test mapping is documented in docs/CHAOS-CRASH-REPLAY.md.
test-validation-chaos:
	./devsetup/test-validation-chaos.sh

# Run the real-runtime worker capacity matrix. Override the fixed iteration
# count when collecting release evidence, for example BENCHTIME=10x.
benchmark-en16931:
	./devsetup/benchmark-en16931.sh

# Run the independent XSD, EN 16931, and orchestrator capacity matrix with 100
# latency observations per case. Override with CAPACITY_SAMPLES=200.
benchmark-stages:
	./devsetup/benchmark-stages.sh

# Run the EN matrix inside a container with the production 2 CPU / 2 GiB hard
# limits. A Kubernetes CPU request is scheduling metadata, not a cgroup limit.
benchmark-stages-cgroup:
	./devsetup/benchmark-stages-cgroup.sh

# Regenerate and verify both tracked corpora, then run them through the real
# authenticated API/worker topology with all required runtimes available.
regression-correctness:
	ARTIFACT_DIR="$(ARTIFACT_DIR)" \
	./devsetup/regression-correctness.sh

# Compare a candidate normalized result set with a reviewed monolithic
# baseline. BASELINE and CANDIDATE are required; OUTPUT is optional.
regression-equivalence:
	@test -n "$(BASELINE)" || { echo "ERROR: BASELINE is required" >&2; exit 2; }
	@test -n "$(CANDIDATE)" || { echo "ERROR: CANDIDATE is required" >&2; exit 2; }
	go run . regression equivalence --baseline "$(BASELINE)" --candidate "$(CANDIDATE)" $(if $(OUTPUT),--output "$(OUTPUT)")

# Build and load the candidate image, deploy the current distributed topology,
# run the existing semantic regression runner, collect diagnostics, and remove
# the disposable namespace. Override MANIFEST, PROFILE, or ARTIFACT_DIR as needed.
regression-minikube:
	MANIFEST="$(or $(MANIFEST),createsamples/samples-en16931/manifest.json)" \
	PROFILE="$(or $(PROFILE),all)" \
	ARTIFACT_DIR="$(ARTIFACT_DIR)" \
	./devsetup/regression-minikube.sh

# Recover from an interrupted run. This removes only the dedicated regression
# namespace; the Minikube profile and local artifacts are retained.
regression-minikube-clean:
	EUBL21_MINIKUBE_PROFILE="$(or $(MINIKUBE_PROFILE),e-ubl21-regression)" \
	./devsetup/minikube-kubectl.sh delete namespace e-ubl21-regression --ignore-not-found=true
