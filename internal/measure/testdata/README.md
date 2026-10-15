# measure fixtures (synthetic)

Every file here is synthetic: written by hand from each export's documented format, because no billing,
Application Insights, AWS, or GCP account exists yet. They are an asymmetric fixture for `aayu measure`
(WP7.2): users acquired through different sources, events on both sides of every window boundary, a
failed and a refunded payment, unparsable rows, an event name that reads like an instruction, and a
cohort week with no users. The expected cohorts are worked out by hand in `internal/cli/measure_test.go`.

- `billing.csv`: a Stripe-style payments export (`Created date (UTC)`, `Amount`, `Amount Refunded`,
  `Currency`, `Status`, `Customer ID`).
- `appinsights.jsonl`: Application Insights events, Log Analytics export rows (`TimeGenerated`, `Name`,
  `UserId`, `UserAuthenticatedId`) and one classic `customEvents` row (`timestamp`, `name`, `user_Id`).
- `aws.jsonl`: CloudWatch RUM events (`event_timestamp` in milliseconds, `event_type`,
  `user_details.userId`).
- `gcp.jsonl`: Google Analytics 4 BigQuery export rows (`event_timestamp` in microseconds, `event_name`,
  `user_id`, `user_pseudo_id`).
