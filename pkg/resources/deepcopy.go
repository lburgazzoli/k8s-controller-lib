package resources

// DeepCopyable is an interface for types that can deep copy themselves.
// This is typically implemented by Kubernetes API types.
type DeepCopyable[T any] interface {
	*T
	DeepCopy() *T
}

// DeepCopySlice creates a deep copy of a slice where each element implements DeepCopy.
// Returns nil if the input slice is nil.
//
// This is useful for copying slices of Kubernetes API types that provide DeepCopy methods.
//
// Example:
//
//	fields := []metav1.ManagedFieldsEntry{...}
//	copied := DeepCopySlice(fields)
//	// copied is a deep copy, modifications won't affect original
func DeepCopySlice[T any, PT DeepCopyable[T]](slice []T) []T {
	if slice == nil {
		return nil
	}

	result := make([]T, len(slice))
	for i := range slice {
		result[i] = *PT(&slice[i]).DeepCopy()
	}

	return result
}
