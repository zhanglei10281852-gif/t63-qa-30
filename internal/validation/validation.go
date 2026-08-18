package validation

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Violation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
type Violations []Violation

func (v Violations) Error() string {
	parts := make([]string, 0, len(v))
	for _, item := range v {
		parts = append(parts, item.Field+": "+item.Message)
	}
	return strings.Join(parts, "; ")
}

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{1,31}$`)
var platePattern = regexp.MustCompile(`^[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼][A-HJ-NP-Z]([A-HJ-NP-Z0-9]{5}|[DF][A-HJ-NP-Z0-9]{5}|[A-HJ-NP-Z0-9]{5}[DF])$`)

// NormalizePlate keeps the value stored and searched by the API canonical.
func NormalizePlate(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "", "-", "", "·", "", ".", "").Replace(value)
	return value
}

type Collector struct{ values Violations }

func (c *Collector) Required(field, value string) {
	if strings.TrimSpace(value) == "" {
		c.values = append(c.values, Violation{Field: field, Message: "is required"})
	}
}
func (c *Collector) MaxLength(field, value string, limit int) {
	if len([]rune(value)) > limit {
		c.values = append(c.values, Violation{Field: field, Message: fmt.Sprintf("must contain at most %d characters", limit)})
	}
}
func (c *Collector) Positive(field string, value int) {
	if value <= 0 {
		c.values = append(c.values, Violation{Field: field, Message: "must be positive"})
	}
}
func (c *Collector) NonNegative(field string, value int) {
	if value < 0 {
		c.values = append(c.values, Violation{Field: field, Message: "must be non-negative"})
	}
}
func (c *Collector) Code(field, value string) {
	if value != "" && !codePattern.MatchString(value) {
		c.values = append(c.values, Violation{Field: field, Message: "must be uppercase letters, numbers, or hyphens"})
	}
}
func (c *Collector) Plate(field, value string) {
	if value != "" && !platePattern.MatchString(NormalizePlate(value)) {
		c.values = append(c.values, Violation{Field: field, Message: "has an invalid format"})
	}
}
func (c *Collector) Future(field string, value, now time.Time) {
	if value.IsZero() || !value.After(now) {
		c.values = append(c.values, Violation{Field: field, Message: "must be in the future"})
	}
}
func (c *Collector) Window(startField, endField string, start, end time.Time, maximum time.Duration) {
	if start.IsZero() {
		c.values = append(c.values, Violation{Field: startField, Message: "is required"})
	}
	if end.IsZero() || !end.After(start) {
		c.values = append(c.values, Violation{Field: endField, Message: "must be after start"})
	} else if maximum > 0 && end.Sub(start) > maximum {
		c.values = append(c.values, Violation{Field: endField, Message: "window is too long"})
	}
}
func (c *Collector) Err() error {
	if len(c.values) == 0 {
		return nil
	}
	result := make(Violations, len(c.values))
	copy(result, c.values)
	return result
}
