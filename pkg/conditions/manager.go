package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Manager manages conditions on an Accessor. It provides methods to set
// individual conditions and compute a top-level condition from contributors.
//
// Create a Manager with NewManager, specifying which conditions contribute
// to the top-level condition and their polarity:
//
//	m := conditions.NewManager(
//	    conditions.ConditionTypeReady,
//	    conditions.PositivePolarity(conditions.ConditionTypeAvailable),
//	    conditions.PositivePolarity("ProvisioningSucceeded"),
//	    conditions.NegativePolarity(conditions.ConditionTypeDegraded),
//	)
//
// Then use it during reconciliation:
//
//	m.MarkTrue(accessor, conditions.ConditionTypeAvailable,
//	    conditions.WithReason("Reconciled"),
//	    conditions.WithObservedGeneration(gen))
//	m.Compute(accessor, gen)
type Manager struct {
	target       string
	contributors []Contributor
}

// Contributor defines a condition that feeds into the top-level condition.
// Use PositivePolarity or NegativePolarity to create contributors.
type Contributor struct {
	Type     string
	Negative bool
}

// PositivePolarity creates a contributor where True means healthy.
func PositivePolarity(conditionType string) Contributor {
	return Contributor{Type: conditionType}
}

// NegativePolarity creates a contributor where True means unhealthy.
func NegativePolarity(conditionType string) Contributor {
	return Contributor{Type: conditionType, Negative: true}
}

// NewManager creates a Manager that computes the target condition
// from the given contributors.
func NewManager(
	target string,
	contributors ...Contributor,
) *Manager {
	return &Manager{
		target:       target,
		contributors: contributors,
	}
}

// MarkTrue sets a condition to True on the accessor.
func (m *Manager) MarkTrue(
	accessor Accessor,
	conditionType string,
	opts ...ConditionOption,
) {
	Set(accessor, conditionType, metav1.ConditionTrue, opts...)
}

// MarkFalse sets a condition to False on the accessor.
func (m *Manager) MarkFalse(
	accessor Accessor,
	conditionType string,
	opts ...ConditionOption,
) {
	Set(accessor, conditionType, metav1.ConditionFalse, opts...)
}

// MarkUnknown sets a condition to Unknown on the accessor.
func (m *Manager) MarkUnknown(
	accessor Accessor,
	conditionType string,
	opts ...ConditionOption,
) {
	Set(accessor, conditionType, metav1.ConditionUnknown, opts...)
}

// Compute derives the target condition from the current state of all
// contributors. Call this at the end of reconciliation after setting
// all contributing conditions.
//
// Logic:
//   - Any contributor missing   → target Unknown (short-circuit)
//   - Any contributor unhealthy → target False (first unhealthy's reason/message)
//   - All contributors healthy  → target True
//
// Polarity determines health: for positive polarity, True is healthy;
// for negative polarity, False is healthy.
func (m *Manager) Compute(
	accessor Accessor,
	generation int64,
) {
	reason := ConditionTypeReady
	message := "All conditions met"
	status := metav1.ConditionTrue

	for _, c := range m.contributors {
		found := Get(accessor, c.Type)
		if found == nil {
			status = metav1.ConditionUnknown
			reason = ReasonConditionMissing
			message = c.Type + " not yet reported"

			break
		}

		healthy := found.Status == metav1.ConditionTrue
		if c.Negative {
			healthy = found.Status == metav1.ConditionFalse
		}

		if !healthy {
			status = metav1.ConditionFalse
			reason = found.Reason
			message = found.Message

			break
		}
	}

	conditions := accessor.GetConditions()

	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:               m.target,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	})

	accessor.SetConditions(conditions)
}
