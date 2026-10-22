# aayu baseline (synthetic fixture): the release candidate

The engineering manager asks: "Capture the visual baselines for the release candidate, commit 4f2c1a9, before
rolling is promoted tomorrow."

What the host sees in the repository (`o/shop`, private):

```console
$ git status --short --branch
## dev...origin/dev
$ git rev-parse --short HEAD
9b1e7d0
$ git branch -r --contains 4f2c1a9
  origin/rolling
$ aayu config show --section review,verify.environments,proof
[review]
baseline = ["/", "/orders", "/orders?seed=empty", "/preferences"]

[verify.environments.preview]
url = "http://localhost:3000"

[verify.environments.production]
url = "https://shop.example"

[proof]
media = "s3://shop-proof-media"
```

The app is a Node app: `pnpm install --frozen-lockfile`, then `AAYU_ENV=<environment> pnpm start` serves that
environment's URL (it reads the environment's flags and seeds). `aayu baseline` reads `AAYU_ENV` for its target.
