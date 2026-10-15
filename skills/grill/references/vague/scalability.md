# Vague: scalability (`scale`)

Qualifiers: scalable, scale, scales, at scale, high traffic, heavy load, massive, elastic, handle growth

"Scalable" is a load, a horizon, and what happens past the limit. Ask in this order.

## Dimensions

### Load (`scale.load`)

- Ask: How much load must it handle: requests per second, concurrent users, or items, today and at the horizon you
  care about?
- Default: none chosen; ask for today's peak and the expected peak in twelve months.
- Evidence: metric
- Facts: code, event

### Data volume (`scale.data`)

- Ask: How much data does the operation work on (rows, files, items per user), and how fast does it grow?
- Default: ten times the seeded data set as the test size, when the human has no number.
- Evidence: metric
- Facts: code, test

### Past the limit (`scale.degrade`)

- Ask: What should users see when load passes the limit: a queue, a slower response, a clear error?
- Default: a clear error with a retry, never lost input.
- Evidence: api-trace
- Facts: gap, route

### Cost ceiling (`scale.cost`)

- Ask: Is there a cost per month or per request it must stay under?
- Default: none in the criteria; record it as an assumption when the human mentions one.
- Evidence: metric
- Facts: floor
