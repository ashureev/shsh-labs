package domain

// Challenge represents a curriculum challenge.
type Challenge struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Hints       []string `json:"hints"`
	HintIndex   int      `json:"-"`
}

// NextHint returns the next available hint for the challenge.
// Returns empty string if no more hints are available.
func (c *Challenge) NextHint() string {
	if c.HintIndex >= len(c.Hints) {
		return ""
	}
	hint := c.Hints[c.HintIndex]
	c.HintIndex++
	return hint
}

// HasHints returns true if there are hints remaining.
func (c *Challenge) HasHints() bool {
	return c.HintIndex < len(c.Hints)
}
