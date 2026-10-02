#!/bin/sh
set -eu
# Acquire exact official resources outside the version-controlled documentation.
DEST=${1:-/tmp/peppol-official-resources}
mkdir -p "$DEST"
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/ae-ae-tdd-1.0.4.zip" "https://docs.peppol.eu/poac/ae/ae-tdd/resources.zip"
printf "%s  %s\n" "56cb018a5ac0500f5e6e9841788c8b62d894942ff820ab5449d90fdeb9981649" "$DEST/ae-ae-tdd-1.0.4.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/ae-pint-ae-1.0.4.zip" "https://docs.peppol.eu/poac/ae/pint-ae/resources.zip"
printf "%s  %s\n" "1e8b0bd595c672ac9d6fc886ddd0eb8027cb39a1d26bebdcb1ada37b7eee491f" "$DEST/ae-pint-ae-1.0.4.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/ae-pint-ae-sb-1.0.4.zip" "https://docs.peppol.eu/poac/ae/pint-ae-sb/resources.zip"
printf "%s  %s\n" "7e6e58f120132dd2d2a6acf21b4a9318f0d07f2685c30ff282fae0e2a6613d15" "$DEST/ae-pint-ae-sb-1.0.4.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/aunz-pint-aunz-1.1.3.zip" "https://docs.peppol.eu/poac/aunz/pint-aunz/resources.zip"
printf "%s  %s\n" "277206b650f22689b6f71de38e7e92619de315625910efb3fd89a4d1e039ea5d" "$DEST/aunz-pint-aunz-1.1.3.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/aunz-pint-aunz-sb-1.1.3.zip" "https://docs.peppol.eu/poac/aunz/pint-aunz-sb/resources.zip"
printf "%s  %s\n" "60e481d13da1cd7d1692333c2e30e6743cd0d401c290fb072e6626e600cea1df" "$DEST/aunz-pint-aunz-sb-1.1.3.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/eu-pint-eu-1.1.1.zip" "https://docs.peppol.eu/poac/eu/pint-eu/resources.zip"
printf "%s  %s\n" "ef9dc731c30df7180c528239efcbc6f13e98483e8a5ad062e4a20377bf8b786f" "$DEST/eu-pint-eu-1.1.1.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/jp-pint-jp-1.1.3.zip" "https://docs.peppol.eu/poac/jp/pint-jp/resources.zip"
printf "%s  %s\n" "2d0bb36c14106649f8c14c3e2377c0bd027fb434bd908afeb06900ee620aa570" "$DEST/jp-pint-jp-1.1.3.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/jp-pint-jp-ntr-1.1.3.zip" "https://docs.peppol.eu/poac/jp/pint-jp-ntr/resources.zip"
printf "%s  %s\n" "53b57443e45c442db6ed36ee03b528d87114b37c82d59b919c6950697affe33d" "$DEST/jp-pint-jp-ntr-1.1.3.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/jp-pint-jp-sb-1.1.3.zip" "https://docs.peppol.eu/poac/jp/pint-jp-sb/resources.zip"
printf "%s  %s\n" "b681345d9b5bb44e2e7c75d45f211b19e01340556f589fd39263b4cc8d4d1d6a" "$DEST/jp-pint-jp-sb-1.1.3.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/my-pint-my-1.3.1.zip" "https://docs.peppol.eu/poac/my/pint-my/resources.zip"
printf "%s  %s\n" "bcfa311ffcb082f535c911d12a6b8537115b55e630c71fa1ccf47272271dd3d8" "$DEST/my-pint-my-1.3.1.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/my-pint-my-sb-1.3.1.zip" "https://docs.peppol.eu/poac/my/pint-my-sb/resources.zip"
printf "%s  %s\n" "a071cddd2c3ab3e9f9a81fa430b06d52c0ef35625d092cd19059939e8730d80f" "$DEST/my-pint-my-sb-1.3.1.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/om-om-tdd-1.0.1.zip" "https://docs.peppol.eu/poac/om/om-tdd/resources.zip"
printf "%s  %s\n" "8750f70b4ab787ee6ead44103a5ab8030487408420d0ef5e943013c8fa904373" "$DEST/om-om-tdd-1.0.1.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/om-pint-om-1.0.1.zip" "https://docs.peppol.eu/poac/om/pint-om/resources.zip"
printf "%s  %s\n" "023f57b9cc13ae641dab0c1726fc8167b86c125837875da98274cc6c0aaf8402" "$DEST/om-pint-om-1.0.1.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/om-pint-om-sb-1.0.1.zip" "https://docs.peppol.eu/poac/om/pint-om-sb/resources.zip"
printf "%s  %s\n" "0f8095fecc15393d792cfddfd4cfc56b58e6fc3588b02b0e7c47df9de5fb532b" "$DEST/om-pint-om-sb-1.0.1.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/sg-pint-sg-1.4.1.zip" "https://docs.peppol.eu/poac/sg/pint-sg/resources.zip"
printf "%s  %s\n" "584353b4fed58474d3ec7e32cc99d24eb8f50c812ac7d4a4ae72e6e0566b2670" "$DEST/sg-pint-sg-1.4.1.zip" | sha256sum --check -
curl --fail --location --proto "=https" --tlsv1.2 --retry 3 --output "$DEST/sg-sg-bis-3.0.zip" "https://www.peppolguide.sg/billing/files/SG-BIS-Billing3-Schematron.zip"
printf "%s  %s\n" "77c7cbf5e686240e364cb88789d3902ff87202444f9a12fe64c35157edb23e51" "$DEST/sg-sg-bis-3.0.zip" | sha256sum --check -
