# Security Policy

## Supported versions

Security fixes are provided for the latest Mobazha release. Pre-release branches and older releases may receive fixes at the maintainers' discretion.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability, leaked credential, signing-key concern, or exploit.

Use GitHub's private vulnerability reporting for this repository:

1. Open the repository's **Security** tab.
2. Select **Advisories** and **Report a vulnerability**.
3. Include affected versions, impact, reproduction steps, and any proposed mitigation.

If private vulnerability reporting is temporarily unavailable, contact the repository owners privately through the Mobazha GitHub organization and ask for a secure reporting channel. Do not include exploit details in the initial public contact.

Maintainers will acknowledge a complete report when practical, coordinate validation and remediation, and credit reporters who request attribution. Please allow time for a fix and release before public disclosure.

## Scope reminders

- Never submit production credentials, private keys, seeds, tokens, private RPC URLs, or customer data.
- Treat chain RPC, indexer, plugin, and webhook inputs as hostile.
- A plugin must not receive raw signing material or bypass Core verification and settlement policy.
