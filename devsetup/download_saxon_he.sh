#!/usr/bin/env bash
set -Eeuo pipefail
#chmod +x download_saxon_he.sh
#./download_saxon_he.sh
# Download Saxon-HE (Home Edition), the XSLT 2.0/3.0 processor used to run
# the official EN16931 UBL validation XSLT.
#
# See ideas/MAIN-EN16931-UBL-validation.md §8/§30/§32: Saxon's version must
# be explicitly pinned, and the runtime should not depend on downloading it
# at request time. This script fetches it once into a local, gitignored
# cache directory (devsetup/tools/saxon/) for local development and CI use;
# it is not committed to the repository as a binary.
#
# Result:
#
# devsetup/tools/saxon/
# ├── saxon-he-12.7.jar
# └── xmlresolver-6.0.17.jar
#
# Usage:
#   chmod +x download_saxon_he.sh
#   ./download_saxon_he.sh
#
# Optional custom destination directory:
#   ./download_saxon_he.sh /path/to/dir
#
# Example transform once downloaded (both jars are required on the
# classpath: Saxon 12.x depends on xmlresolver at runtime):
#   java -cp "devsetup/tools/saxon/saxon-he-12.7.jar:devsetup/tools/saxon/xmlresolver-6.0.17.jar" \
#       net.sf.saxon.Transform \
#       -s:invoice.xml \
#       -xsl:validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt \
#       -o:en16931.svrl.xml

SAXON_VERSION="12.7"
JAR_NAME="saxon-he-${SAXON_VERSION}.jar"
MAVEN_URL="https://repo1.maven.org/maven2/net/sf/saxon/Saxon-HE/${SAXON_VERSION}/Saxon-HE-${SAXON_VERSION}.jar"

# Known-good SHA-1 for Saxon-HE 12.7.jar as published on Maven Central,
# recorded when this script was authored. A mismatch means the artefact
# changed unexpectedly (Maven Central releases are normally immutable) and
# must be investigated before trusting the download.
EXPECTED_SHA1="202a9133dc7fe8344e7c729e6d515d17b1ddc66a"

# Saxon 12.x requires org.xmlresolver:xmlresolver on the classpath at
# runtime (it is used for XML catalog/URI resolution even when no catalog
# file is configured); without it, net.sf.saxon.Transform fails immediately
# with java.lang.NoClassDefFoundError: org/xmlresolver/Resolver.
RESOLVER_VERSION="6.0.17"
RESOLVER_JAR_NAME="xmlresolver-${RESOLVER_VERSION}.jar"
RESOLVER_MAVEN_URL="https://repo1.maven.org/maven2/org/xmlresolver/xmlresolver/${RESOLVER_VERSION}/xmlresolver-${RESOLVER_VERSION}.jar"
EXPECTED_RESOLVER_SHA1="5b7642c6a854b275f68324361a239b824040bdf2"

DEST_DIR="${1:-devsetup/tools/saxon}"

if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl is required but was not found." >&2
    exit 1
fi

if ! command -v sha1sum >/dev/null 2>&1; then
    echo "ERROR: sha1sum is required but was not found." >&2
    exit 1
fi

mkdir -p "$DEST_DIR"

download_and_verify() {
    local url="$1"
    local dest_name="$2"
    local expected_sha1="$3"

    echo
    echo "Downloading ${dest_name}..."
    curl -sSL -o "$DEST_DIR/$dest_name" "$url"

    echo "Verifying checksum for ${dest_name}..."
    local actual_sha1
    actual_sha1="$(sha1sum "$DEST_DIR/$dest_name" | awk '{print $1}')"

    if [[ "$actual_sha1" != "$expected_sha1" ]]; then
        echo "ERROR: checksum mismatch for $dest_name." >&2
        echo "  expected: $expected_sha1" >&2
        echo "  actual:   $actual_sha1" >&2
        exit 1
    fi
}

download_and_verify "$MAVEN_URL" "$JAR_NAME" "$EXPECTED_SHA1"
download_and_verify "$RESOLVER_MAVEN_URL" "$RESOLVER_JAR_NAME" "$EXPECTED_RESOLVER_SHA1"

CLASSPATH="$DEST_DIR/$JAR_NAME:$DEST_DIR/$RESOLVER_JAR_NAME"

echo
echo "Saxon-HE ${SAXON_VERSION} (+ xmlresolver ${RESOLVER_VERSION}) download complete:"
echo "  $DEST_DIR/$JAR_NAME"
echo "  $DEST_DIR/$RESOLVER_JAR_NAME"
echo
echo "Example transform (both jars are required on the classpath):"
echo "  java -cp \"$CLASSPATH\" net.sf.saxon.Transform \\"
echo "      -s:invoice.xml \\"
echo "      -xsl:validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt \\"
echo "      -o:en16931.svrl.xml"
