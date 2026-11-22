package domain

import "fmt"

// ReviewDriverType is the provider that hosts the pull requests and their
// reviews. NoReview keeps pull requests outside Paracell.
type ReviewDriverType string

const (
	NoReview     ReviewDriverType = "none"
	GitHubReview ReviewDriverType = "github"
)

func NewReviewDriverType(value string) (ReviewDriverType, error) {
	driver := ReviewDriverType(value)
	switch driver {
	case "":
		return NoReview, nil
	case NoReview, GitHubReview:
		return driver, nil
	default:
		return driver, fmt.Errorf("invalid review driver type %q", driver)
	}
}

// Enabled reports whether Paracell opens and follows the pull requests.
func (d ReviewDriverType) Enabled() bool {
	return d != "" && d != NoReview
}
