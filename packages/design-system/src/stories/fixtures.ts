// Seeded, synthetic data for stories (no real people or claims).
export type ClaimStatus = "Approved" | "Rejected" | "In review" | "Overdue" | "Draft"

export type Claim = {
  id: string
  employee: string
  plan: "Dental" | "Vision" | "Wellness"
  amount: number
  submitted: string
  status: ClaimStatus
}

export const statusVariant = {
  Approved: "positive",
  Rejected: "negative",
  "In review": "informative",
  Overdue: "critical",
  Draft: "neutral",
} as const satisfies Record<ClaimStatus, string>

export const claims: Claim[] = [
  { id: "C-1042", employee: "Asha Rao", plan: "Dental", amount: 180, submitted: "2026-10-03", status: "In review" },
  { id: "C-1041", employee: "Jonas Meyer", plan: "Vision", amount: 249.5, submitted: "2026-10-02", status: "Approved" },
  { id: "C-1040", employee: "Mei Lin", plan: "Wellness", amount: 60, submitted: "2026-10-01", status: "Overdue" },
  { id: "C-1039", employee: "Kofi Mensah", plan: "Dental", amount: 920, submitted: "2026-09-29", status: "Rejected" },
  { id: "C-1038", employee: "Lucía Gómez", plan: "Vision", amount: 135, submitted: "2026-09-28", status: "Draft" },
  { id: "C-1037", employee: "Arjun Nair", plan: "Wellness", amount: 45, submitted: "2026-09-27", status: "Approved" },
  { id: "C-1036", employee: "Sara Kowalski", plan: "Dental", amount: 310, submitted: "2026-09-25", status: "In review" },
  { id: "C-1035", employee: "Tomás Silva", plan: "Vision", amount: 199, submitted: "2026-09-24", status: "Approved" },
]
