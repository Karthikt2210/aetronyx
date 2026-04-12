# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| v0.x    | Yes — best effort while in pre-release development |

No stable release has been tagged yet. Once v1.0 is released, this table will be updated with an explicit support window.

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities by emailing the maintainer directly. You will receive an acknowledgement within 72 hours and a resolution timeline within 7 days.

For GPG-encrypted reports, request the public key in your initial email.

## Disclosure policy

- We follow coordinated disclosure. Please give us 90 days to fix and release before public disclosure.
- Credit will be given in the changelog and release notes unless you request otherwise.
- We do not offer a bug bounty at this time.

## Scope

In scope:
- Prompt injection that causes unintended file writes outside the workspace
- Authentication or authorization bypasses in the local HTTP server
- Audit log tampering or chain-of-custody breaks
- Credential exposure (BYOK API keys leaking to logs or network)

Out of scope:
- Vulnerabilities in third-party LLM providers (report to them directly)
- Social engineering
- Attacks requiring physical access to the machine

## Future compliance documentation

Detailed security controls and compliance mappings will be published in [`docs/compliance/`](./docs/compliance/) starting in Milestone 5.
