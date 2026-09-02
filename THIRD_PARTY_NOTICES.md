# Third-party notices for standards validation runtimes

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

## Peppol BIS Billing 3.0.21 compiled validation artefact

- Component: `PEPPOL-EN16931-UBL-validation.xslt`
- Source: `OpenPEPPOL/peppol-bis-invoice-3`, immutable commit
  `806866bd2bd91d7e9623b68f08164e8fbe9e67a0`
- Source Schematron SHA-256:
  `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`
- Packaged XSLT SHA-256:
  `56bbd05ea26b4ffb859e0c15fcd2c1c8782870b530aae06a828ddb0bdf9b66aa`
- Compiler: SchXslt 1.10.1, MIT License
- Compiler release: <https://codeberg.org/SchXslt/schxslt/releases/tag/v1.10.1>
- Compiler license: <https://codeberg.org/SchXslt/schxslt/src/tag/v1.10.1/LICENSE>

The XSLT is generated deterministically from the pinned OpenPeppol Schematron.
The OpenPeppol/CEN notices in that source continue to apply. SchXslt is used
only during build/release verification, but portions of its MIT-licensed
templates are represented in the generated XSLT.

SchXslt MIT license:

> Copyright (c) 2018-2021 David Maus
>
> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to deal
> in the Software without restriction, including without limitation the rights
> to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
> copies of the Software, and to permit persons to whom the Software is
> furnished to do so, subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in all
> copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
> SOFTWARE.

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
