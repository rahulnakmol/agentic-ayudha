# Vague: security (`sec`)

Qualifiers: secure, securely, safe, safely, private, privacy, protected, hardened, trustworthy, compliant, locked down

The SAST, secrets, dependency, and (when on) DAST floors apply to every story. "Secure" in a request means a
specific asset protected from a specific actor; ask which.

## Dimensions

### Asset and actor (`sec.threat`)

- Ask: What must be protected (an account, payment details, personal data, an admin action), and from whom
  (another user, an anonymous visitor, a compromised device)?
- Default: none chosen; this one needs the human's answer, because a guessed threat produces the wrong control.
- Evidence: api-trace
- Facts: route, code
- Topics: asset-and-actor

### Who may do what (`sec.access`)

- Ask: Who may see or change this, and what happens to someone who may not (a 403, a redirect to sign in)?
- Default: the access rules the product already applies to similar routes, quoted from the code.
- Evidence: api-trace
- Facts: route, code, test
- Topics: who-may, denial

### Personal data (`sec.data`)

- Ask: Which personal data is stored, shown, or logged? Must any of it be masked, encrypted, or deleted after a
  time?
- Default: no new personal data stored, and none in logs.
- Evidence: api-trace
- Facts: code, event
- Topics: personal-data, logging, retention

### Abuse limits (`sec.abuse`)

- Ask: Can someone repeat the action to harm others (guessing codes, scraping, spamming)? Is there a limit per user
  or per minute?
- Default: none in this story unless the human names an abuse case; park it as an idea otherwise.
- Evidence: api-trace
- Facts: route
- Topics: rate-limit

### Standard (`sec.standard`)

- Ask: Is there a standard or regulation this must meet (PCI DSS, GDPR, SOC 2), and which requirement of it?
- Default: none beyond the floors.
- Evidence: api-trace
- Facts: floor
- Topics: standard
