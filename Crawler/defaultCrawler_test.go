package crawler

import "testing"

// helper_test.go
func TestFormatPrice(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int
		expectError bool
	}{
		{
			name:     "simple price",
			input:    "$100",
			expected: 100,
		},
		{
			name:     "price with commas",
			input:    "$1,234",
			expected: 1234,
		},
		{
			name:     "price with cents (truncated)",
			input:    "$99.99",
			expected: 99,
		},
		{
			name:     "price with newlines",
			input:    "\n$50\n",
			expected: 50,
		},
		{
			name:     "price with cents",
			input:    "\n$1,299.00\n",
			expected: 1299,
		},
		{
			name:        "non int string",
			input:       "located in sth",
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatPrice(tt.input)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && result != tt.expected {
				t.Errorf("formatPrice(%q) = %d, want %d",
					tt.input, result, tt.expected)
			}
		})
	}
}
