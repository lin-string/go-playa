# Security policy

## Supported versions

Security fixes target the latest published go-playa release and the current
default branch. Before the first release, only the current default branch is
supported. Older releases and branches do not receive guaranteed backports;
please upgrade to the latest release when a fix is available. Reports affecting
older versions are still welcome, especially when they reproduce on a supported
version.

## Report a vulnerability privately

Prefer [GitHub private vulnerability reporting](https://github.com/lin-string/go-playa/security/advisories/new).
If that channel is unavailable, email
[linshijun.string@gmail.com](mailto:linshijun.string@gmail.com) as a fallback.
Do not open a public issue or pull request containing an undisclosed exploit or
a sensitive PDF.

Include the affected version or commit, Go version and operating system, a
minimal reproduction, the observed impact, and the expected behavior. A small,
sanitized PDF or a generator is helpful; do not include confidential documents,
credentials, or personal data. For resource-exhaustion reports, include input
size, resource limits, and observed time or memory use when available.

This is a personally maintained project. The maintainer aims to acknowledge
reports within seven calendar days and, after acknowledgement, provide an update
at least every fourteen calendar days while an accepted report is under active
investigation. These are best-effort targets, not guarantees, service-level
agreements, or 24/7 coverage. There is no guaranteed resolution date. If no
acknowledgement arrives after seven days, a follow-up through the fallback
contact is welcome.

## Coordinated disclosure

Please allow time to reproduce the issue, assess its impact, and prepare a fix
before publishing exploit details. The maintainer and reporter should agree on
a disclosure timeline and revisit it when circumstances change. When a fix is
available, the project aims to publish affected versions, upgrade guidance, and
a security advisory, with reporter credit if desired. Coordination does not
require indefinite secrecy.

## Hostile PDF boundary

PDF input, embedded fonts, images, streams, and metadata can be hostile. Parsers
should report malformed input without panics, unbounded expansion, or uncontrolled
recursion. Failures of those protections are worth reporting privately when they
have a security impact. Recovery and compatibility behavior do not excuse denial
of service.

go-playa is a parsing library, not a security sandbox, and does not promise
perfect safety for arbitrary input. Cache budgets bound retained cache entries;
they are not a hard limit on total process memory, decoding work, or elapsed time.
Applications handling untrusted PDFs should apply input-size limits and run work
with process-level memory, CPU, and time limits, appropriate isolation, and
minimal privileges. Extracted text, links, actions, and other document values
remain untrusted data for the consuming application.

See the [engineering conventions](../docs/engineering.md) and
[compatibility and corpus workflow](../docs/compatibility.md) for the tested
behavior and acceptance boundary.
