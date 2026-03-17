package manifest

// DeleteOrder returns resource kinds in reverse dependency order for safe deletion.
func DeleteOrder() []ResourceKind {
	order := ApplyOrder()
	reversed := make([]ResourceKind, len(order))
	for i, k := range order {
		reversed[len(order)-1-i] = k
	}
	return reversed
}
