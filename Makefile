.PHONY: test-en16931 benchmark-en16931

# Run positive and targeted-negative EN 16931 corpora, plus the XSD
# short-circuit control, with runtime skipping disabled.
test-en16931:
	./devsetup/test-en16931.sh

# Run the real-runtime worker capacity matrix. Override the fixed iteration
# count when collecting release evidence, for example BENCHTIME=10x.
benchmark-en16931:
	./devsetup/benchmark-en16931.sh
