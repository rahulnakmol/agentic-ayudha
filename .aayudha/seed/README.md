# Seed data for proof captures

`aayu verify` captures screenshots, recordings, and screen-reader transcripts of the running app. Those captures may
only show seeded, synthetic data (docs/BUILD-PLAN.md V3, WP4.5). Put the command that loads it here and name it in
`.aayudha/config.toml`:

```toml
[verify]
seed = ["./.aayudha/seed/load.sh"]   # argv; runs once before the first capture

[verify.environments.local]
url = "http://localhost:3000"        # AAYU_ENV picks the environment (default local)

[verify.environments.prod]
url = "https://app.example.com"
production = true                    # verify refuses to capture or seed it (exit 4)
```

Use reserved example domains (`example.com`, `*.test`) for email addresses. The redaction pass blocks a pack
whose captures show any other address, a payment card number, or a national ID number.
