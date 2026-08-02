# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.0.x | Active |

## Reporting a Vulnerability

If you discover a security vulnerability in FuuDelivery, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

### Contact

- **Email**: security@fuudelivery.com.br (or open a private GitHub issue with the `security` label)
- **Response time**: We aim to acknowledge within 48 hours
- **Resolution time**: Critical vulnerabilities within 7 days, others within 30 days

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Security Measures

### Authentication and Authorization
- JWT tokens with signing method validation
- bcrypt password hashing
- Role-based access control (client, delivery, establishment, admin)

### Rate Limiting
| Endpoint | Limit |
|----------|-------|
| Login | 5 requests/min |
| Payment creation | 10 requests/min |
| Wallet operations | 20 requests/min |

### Data Protection
- CORS restricted to known domains via ALLOWED_ORIGINS
- No secrets committed to git (monitored via CI)
- Environment variables for all sensitive configuration

### Payment Security
- Webhook signature verification (AbacatePay)
- Atomic wallet operations (MongoDB $inc) to prevent race conditions
- Idempotency checks via TransactionExistsByReference
- Risk scoring engine (4 factors) for automatic approval/rejection

### Infrastructure
- HTTPS enforced on all Render services
- Redis queue with fallback to in-memory channels (dev only)
- Health checks on all services (/health endpoint)
- CI pipeline with govulncheck and npm audit

## Scope

In scope: FuuDelivery API, frontend applications, payment processing, authentication flows, data storage.

Out of scope: Third-party services (Render, AbacatePay, Supabase, MongoDB Atlas), social engineering, physical attacks.

## Disclosure Policy

We follow responsible disclosure: no legal action against researchers, public credit in advisories, no disclosure until fix is available.

---

*Last updated: August 2, 2026*
