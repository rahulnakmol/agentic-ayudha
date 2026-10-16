# quotes-api

Synthetic eval fixture: a shipping-quotes HTTP API. Partner shops call `POST /quotes` at checkout with up to 10
parcels and show the total to their customer. `GET /quotes/{id}` fetches a stored quote. The contract is
`openapi.yaml`.

Partners say quotes are slow at checkout. Nobody has measured it here: there is no load test and no latency budget.

Run it with `npm start`; test it with `npm test`.
