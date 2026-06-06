package worksection

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// NewLimiter parses a CLI rate-limit spec such as 1/s.
func NewLimiter(spec string) (*rate.Limiter, error) {
	if spec == "" {
		spec = "1/s"
	}
	parts := strings.Split(spec, "/")
	if len(parts) != 2 || parts[1] != "s" {
		return nil, fmt.Errorf("unsupported rate limit %q, expected N/s", spec)
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 {
		return nil, fmt.Errorf("unsupported rate limit %q, expected N/s", spec)
	}
	return rate.NewLimiter(rate.Every(time.Second/time.Duration(n)), 1), nil
}
