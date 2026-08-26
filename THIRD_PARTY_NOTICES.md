# Third-party notices for the EN 16931 validation runtime

The production image redistributes the following pinned validation components.
The component licenses apply to those components; they do not replace the
license of e-ubl21 itself. This inventory is informational and is not legal
advice.

## EN 16931 UBL validation artefact 1.3.16

- Component: `EN16931-UBL-validation.xslt`
- Source: `ConnectingEurope/eInvoicing-EN16931`, release
  `validation-1.3.16`
- Packaged SHA-256:
  `39f9d282867f1a49e7708d9e29a53da89643e1ee56f10cec1ebcf1277595fcbd`
- License: European Union Public Licence 1.2 (`EUPL-1.2`)
- License text: <https://joinup.ec.europa.eu/collection/eupl/eupl-text-eupl-12>
- Exact source and provenance:
  `validation-assets/en16931/1.3.16/source-url.txt`

The stylesheet carries its EUPL 1.2 declaration in its source header and is
redistributed unchanged.

## Saxon-HE 12.7

- Component: `Saxon-HE-12.7.jar`, packaged as `saxon-he-12.7.jar`
- Publisher: Saxonica
- Packaged SHA-256:
  `c607e8b395d4ee237495ad163183971ca3453d491534c847ebb55e0462772086`
- License: Mozilla Public License 2.0 (`MPL-2.0`)
- License text: <https://www.mozilla.org/MPL/2.0/>
- Upstream release: <https://github.com/Saxonica/Saxon-HE/releases/tag/SaxonHE12-7>
- Corresponding source distribution: `saxon12-7source.zip` on that release

Saxon-HE is redistributed as the unmodified upstream binary downloaded from
Maven Central and verified before packaging.

## XML Resolver 6.0.17

- Component: `org.xmlresolver:xmlresolver:6.0.17`
- Publisher: Norman Walsh / XML Resolver project
- Packaged SHA-256:
  `b52e89a81fe86161bf2422997c041856791b3a78dd13824d958170b443b88c56`
- License: Apache License 2.0 (`Apache-2.0`)
- License text: <https://www.apache.org/licenses/LICENSE-2.0.txt>
- Upstream source: <https://github.com/xmlresolver/xmlresolver/tree/6.0.17>

XML Resolver is redistributed as the unmodified Maven Central binary. Its
tagged build metadata declares the Apache License version 2.0.

## Runtime packages

The Alpine base image, OpenJDK 21 runtime, libxml2/xmllint, and CA certificate
packages retain their respective package licenses and notices. An image-level
software inventory can be obtained from the package database with:

```sh
docker run --rm --entrypoint apk e-ubl21:latest info -vv
```
