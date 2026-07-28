# Security Policy

## Supported versions

Security fixes are applied on the latest `main` branch and released through the normal deploy pipeline for [twintube.site](https://twintube.site).

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security problems.

Email the maintainer privately (account used for [buymeacoffee.com/emireln](https://buymeacoffee.com/emireln) / GitHub [@emireln](https://github.com/emireln)), and include:

- A short description of the issue
- Impact (auth bypass, XSS, room takeover, etc.)
- Steps to reproduce or a minimal PoC
- Affected version / commit if known

You should receive an acknowledgement within a few days. Please allow time to investigate and ship a fix before any public disclosure.

## Scope notes

- Guest rooms are intentionally ephemeral (in-memory + short grace period).
- Production deployments must set a strong `JWT_SECRET` and never commit real `.env` files.
- TURN credentials (`TURN_SECRET`) are sensitive; treat them like production secrets.
