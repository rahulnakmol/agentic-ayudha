// Package policy decides what the benefits-qa example may answer (layer L04,
// docs/BUILD-PLAN.md WP8.1). Before retrieval, Screen refuses questions about
// another person's records and requests for medical or financial advice.
// Another person's records are any record word (claim, salary, diagnosis,
// prescription, balance, …) in a question that also refers to someone else:
// a third-person pronoun, a person word, or a possessive other than that of a
// thing (the employer, a plan, a period). After retrieval, Ground lets an
// answer through only when it quotes a plan passage that covers enough of the
// question; otherwise the question is referred to an HR specialist. The rules
// are deterministic.
package policy

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Outcomes.
const (
	Answer = "answer" // quote the cited passage
	Refer  = "refer"  // not in the plan documents: an HR specialist answers
	Refuse = "refuse" // the example must not answer it at all
)

// Reasons.
const (
	ReasonEmpty      = "empty"
	ReasonTooLong    = "too-long"
	ReasonPersonal   = "personal-data"
	ReasonAdvice     = "advice"
	ReasonNotInPlans = "not-in-plan-documents"
)

// MaxQuestion bounds a question, in characters.
const MaxQuestion = 500

// MinScore is the share of the question's terms a cited passage must hold
// for the answer to count as grounded.
const MinScore = 0.5

// Verdict is a policy decision.
type Verdict struct {
	Outcome string
	Reason  string // empty for Answer
	Message string // what the asker is told when the outcome is not Answer
}

var (
	// recordWord names a personal record.
	recordWord = regexp.MustCompile(`(?i)\b(claims?|records?|salary|salaries|pay|payslips?|diagnos[ie]s|prescriptions?|balances?|history|enrol(l)?ments?)\b`)
	// thirdParty refers to someone other than the asker: a third-person
	// pronoun, a person word, or a possessive (after notAPerson is removed).
	thirdParty = regexp.MustCompile(`(?i)\b(his|her|hers|their|theirs|someone|somebody|anyone|anybody|another|others?|colleagues?|co-?workers?|managers?|boss|employees?|team ?mates?|spouses?|wife|husband|partners?|sons?|daughters?|child|children|kids?|parents?|mother|father|friends?)\b|\b\w+('s|’s|s')`)
	// notAPerson are possessives of things, not people: the employer, a plan, a period.
	notAPerson = regexp.MustCompile(`(?i)\b(Example Co|plan|policy|company|employer|insurer|provider|account|fund|today|tomorrow|year|month|week)('s|’s)`)
	// Medical or financial advice the example must not give.
	advice = regexp.MustCompile(`(?i)\b(should i (take|stop|start|buy|sell|invest|have|get|choose|pick|enrol|enroll|switch)|(which|what) (plan|option|fund|medication|medicine|dose|dosage) (should i|is (best|better))|is it (better|worth|safe)|do you recommend|what dose|dosage|diagnose)\b`)
)

var messages = map[string]string{
	ReasonEmpty:      "Type a question about your Example Co benefits.",
	ReasonTooLong:    "Shorten your question to 500 characters or fewer.",
	ReasonPersonal:   "We cannot answer questions about another person's records. Only that person or HR can see them.",
	ReasonAdvice:     "We cannot give medical or financial advice. The plan documents and an HR specialist can explain your options.",
	ReasonNotInPlans: "The plan documents do not answer this. An HR specialist will reply within 2 business days.",
}

// Screen checks a question before any retrieval.
func Screen(question string) Verdict {
	q := strings.TrimSpace(question)
	switch {
	case q == "":
		return refuse(ReasonEmpty)
	case utf8.RuneCountInString(q) > MaxQuestion:
		return refuse(ReasonTooLong)
	case recordWord.MatchString(q) && thirdParty.MatchString(notAPerson.ReplaceAllString(q, "")):
		return refuse(ReasonPersonal)
	case advice.MatchString(q):
		return refuse(ReasonAdvice)
	}
	return Verdict{Outcome: Answer}
}

// Ground decides whether a retrieved answer may be shown: it must cite at
// least one passage, and that passage must hold at least MinScore of the
// question's terms. Anything else is referred to HR.
func Ground(score float64, citations int) Verdict {
	if citations > 0 && score >= MinScore {
		return Verdict{Outcome: Answer}
	}
	return Verdict{Outcome: Refer, Reason: ReasonNotInPlans, Message: messages[ReasonNotInPlans]}
}

func refuse(reason string) Verdict {
	return Verdict{Outcome: Refuse, Reason: reason, Message: messages[reason]}
}
