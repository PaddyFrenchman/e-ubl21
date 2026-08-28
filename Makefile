.PHONY: test-en16931 test-en16931-compliance test-en16931-image benchmark-en16931 regression-minikube regression-minikube-clean

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

# Run the real-runtime worker capacity matrix. Override the fixed iteration
# count when collecting release evidence, for example BENCHTIME=10x.
benchmark-en16931:
	./devsetup/benchmark-en16931.sh

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
