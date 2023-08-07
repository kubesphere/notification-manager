package utils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
)

// Operator represents a key/field's relationship to value(s).
// See labels.Requirement and fields.Requirement for more details.
type Operator string

const (
	DoesNotExist Operator = "!"
	Equals       Operator = "="
	DoubleEquals Operator = "=="
	In           Operator = "in"
	NotEquals    Operator = "!="
	NotIn        Operator = "notin"
	Exists       Operator = "exists"
	GreaterThan  Operator = "gt"
	LessThan     Operator = "lt"
	Match        Operator = "match"
)

type Requirement struct {
	key      string
	operator Operator
	// In huge majority of cases we have at most one value here.
	// It is generally faster to operate on a single-element slice
	// than on a single-element map, so we have a slice here.
	strValues []string
}

type internalSelector []Requirement

// ByKey sorts requirements by key to obtain deterministic parser
type ByKey []Requirement

func (a ByKey) Len() int { return len(a) }

func (a ByKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (a ByKey) Less(i, j int) bool { return a[i].key < a[j].key }

// Requirements is AND of all requirements.
type Requirements []Requirement

var (
	validRequirementOperators = []string{
		string(In), string(NotIn),
		string(Equals), string(DoubleEquals), string(NotEquals),
		string(Exists), string(DoesNotExist),
		string(GreaterThan), string(LessThan),
		string(Match),
	}
)

type Selector interface {
	// Matches returns true if this selector matches the given set of labels.
	Matches(labels.Labels) bool

	// Empty returns true if this selector does not restrict the selection space.
	Empty() bool

	// String returns a human readable string that represents this selector.
	String() string

	// Add adds requirements to the Selector
	Add(r ...Requirement) Selector

	// Requirements converts this interface into Requirements to expose
	// more detailed selection information.
	// If there are querying parameters, it will return converted requirements and selectable=true.
	// If this selector doesn't want to select anything, it will return selectable=false.
	Requirements() (requirements Requirements, selectable bool)

	// Make a deep copy of the selector.
	DeepCopySelector() Selector

	// RequiresExactMatch allows a caller to introspect whether a given selector
	// requires a single specific label to be set, and if so returns the value it
	// requires.
	RequiresExactMatch(label string) (value string, found bool)
}

type nothingSelector struct{}

func (n nothingSelector) Matches(_ labels.Labels) bool       { return false }
func (n nothingSelector) Empty() bool                        { return false }
func (n nothingSelector) String() string                     { return "" }
func (n nothingSelector) Add(_ ...Requirement) Selector      { return n }
func (n nothingSelector) Requirements() (Requirements, bool) { return nil, false }
func (n nothingSelector) DeepCopySelector() Selector         { return n }
func (n nothingSelector) RequiresExactMatch(label string) (value string, found bool) {
	return "", false
}

// LabelSelectorAsSelector converts the LabelSelector api type into a struct that implements
// labels.Selector
// Note: This function should be kept in sync with the selector methods in pkg/labels/selector.go
func LabelSelectorAsSelector(ps *v2beta2.LabelSelector) (Selector, error) {
	if ps == nil {
		return Nothing(), nil
	}
	if len(ps.MatchLabels)+len(ps.MatchExpressions) == 0 {
		return Everything(), nil
	}
	selector := NewSelector()
	for k, v := range ps.MatchLabels {
		r, err := NewRequirement(k, Equals, []string{v})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*r)
	}
	for _, expr := range ps.MatchExpressions {
		var op Operator
		switch expr.Operator {
		case v2beta2.LabelSelectorOpIn:
			op = In
		case v2beta2.LabelSelectorOpNotIn:
			op = NotIn
		case v2beta2.LabelSelectorOpExists:
			op = Exists
		case v2beta2.LabelSelectorOpDoesNotExist:
			op = DoesNotExist
		case v2beta2.LabelSelectorOpMatch:
			op = Match
		default:
			return nil, fmt.Errorf("%q is not a valid pod selector operator", expr.Operator)
		}
		if op== Match {
			expr.Values =append([]string(nil), expr.RegexValue)
		}
		r, err := NewRequirement(expr.Key, op, append([]string(nil), expr.Values...))
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*r)
	}
	return selector, nil
}

// NewRequirement is the constructor for a Requirement.
// If any of these rules is violated, an error is returned:
// (1) The operator can only be In, NotIn, Equals, DoubleEquals, NotEquals, Exists, DoesNotExist, Match, or NotMatch.
// (2) If the operator is In or NotIn, the values set must be non-empty.
// (3) If the operator is Equals, DoubleEquals, or NotEquals, the values set must contain one value.
// (4) If the operator is Exists or DoesNotExist, the value set must be empty.
// (5) If the operator is Gt or Lt, the values set must contain only one value, which will be interpreted as an integer.
// (6) The key is invalid due to its length, or sequence
//
//	of characters. See validateLabelKey for more details.
//
// The empty string is a valid value in the input values set.
// Returned error, if not nil, is guaranteed to be an aggregated field.ErrorList
func NewRequirement(key string, op Operator, vals []string, opts ...field.PathOption) (*Requirement, error) {
	var allErrs field.ErrorList
	path := field.ToPath(opts...)
	if err := validateLabelKey(key, path.Child("key")); err != nil {
		allErrs = append(allErrs, err)
	}

	valuePath := path.Child("values")
	switch op {
	case In, NotIn:
		if len(vals) == 0 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "for 'in', 'notin' operators, values set can't be empty"))
		}
	case Equals, DoubleEquals, NotEquals:
		if len(vals) != 1 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "exact-match compatibility requires one single value"))
		}
	case Exists, DoesNotExist:
		if len(vals) != 0 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "values set must be empty for exists and does not exist"))
		}
	case Match:
		if len(vals) == 0 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "For regular expressions, values set can't be empty"))
		}
	case GreaterThan, LessThan:
		if len(vals) != 1 {
			allErrs = append(allErrs, field.Invalid(valuePath, vals, "for 'Gt', 'Lt' operators, exactly one value is required"))
		}
		for i := range vals {
			if _, err := strconv.ParseInt(vals[i], 10, 64); err != nil {
				allErrs = append(allErrs, field.Invalid(valuePath.Index(i), vals[i], "for 'Gt', 'Lt' operators, the value must be an integer"))
			}
		}
	default:
		allErrs = append(allErrs, field.NotSupported(path.Child("operator"), op, validRequirementOperators))
	}

	for i := range vals {
		if err := validateLabelValue(key, vals[i], valuePath.Index(i)); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return &Requirement{key: key, operator: op, strValues: vals}, allErrs.ToAggregate()
}

func validateLabelKey(k string, path *field.Path) *field.Error {
	if errs := validation.IsQualifiedName(k); len(errs) != 0 {
		return field.Invalid(path, k, strings.Join(errs, "; "))
	}
	return nil
}

func validateLabelValue(k, v string, path *field.Path) *field.Error {
	if errs := validation.IsValidLabelValue(v); len(errs) != 0 {
		return field.Invalid(path.Key(k), v, strings.Join(errs, "; "))
	}
	return nil
}

func NewSelector() Selector {
	return internalSelector(nil)
}

// Add adds requirements to the selector. It copies the current selector returning a new one
func (s internalSelector) Add(reqs ...Requirement) Selector {
	var ret internalSelector
	for ix := range s {
		ret = append(ret, s[ix])
	}
	for _, r := range reqs {
		ret = append(ret, r)
	}
	sort.Sort(ByKey(ret))
	return ret
}

func (s internalSelector) Requirements() (Requirements, bool) { return Requirements(s), true }

func (s internalSelector) Empty() bool {
	if s == nil {
		return true
	}
	return len(s) == 0
}

// Matches for a internalSelector returns true if all
// its Requirements match the input Labels. If any
// Requirement does not match, false is returned.
func (s internalSelector) Matches(l labels.Labels) bool {
	for ix := range s {
		if matches := s[ix].Matches(l); !matches {
			return false
		}
	}
	return true
}

func (r *Requirement) hasValue(value string) bool {
	for i := range r.strValues {
		if r.strValues[i] == value {
			return true
		}
	}
	return false
}

// Matches returns true if the Requirement matches the input Labels.
// There is a match in the following cases:
// (1) The operator is Exists and Labels has the Requirement's key.
// (2) The operator is In, Labels has the Requirement's key and Labels'
//
//	value for that key is in Requirement's value set.
//
// (3) The operator is NotIn, Labels has the Requirement's key and
//
//	Labels' value for that key is not in Requirement's value set.
//
// (4) The operator is DoesNotExist or NotIn and Labels does not have the
//
//	Requirement's key.
//
// (5) The operator is GreaterThanOperator or LessThanOperator, and Labels has
//
//	the Requirement's key and the corresponding value satisfies mathematical inequality.
func (r *Requirement) Matches(ls labels.Labels) bool {
	switch r.operator {
	case In, Equals, DoubleEquals:
		if !ls.Has(r.key) {
			return false
		}
		return r.hasValue(ls.Get(r.key))
	case NotIn, NotEquals:
		if !ls.Has(r.key) {
			return true
		}
		return !r.hasValue(ls.Get(r.key))
	case Exists:
		return ls.Has(r.key)
	case DoesNotExist:
		return !ls.Has(r.key)
	case GreaterThan, LessThan:
		if !ls.Has(r.key) {
			return false
		}
		lsValue, err := strconv.ParseInt(ls.Get(r.key), 10, 64)
		if err != nil {
			klog.V(10).Infof("ParseInt failed for value %+v in label %+v, %+v", ls.Get(r.key), ls, err)
			return false
		}

		// There should be only one strValue in r.strValues, and can be converted to an integer.
		if len(r.strValues) != 1 {
			klog.V(10).Infof("Invalid values count %+v of requirement %#v, for 'Gt', 'Lt' operators, exactly one value is required", len(r.strValues), r)
			return false
		}

		var rValue int64
		for i := range r.strValues {
			rValue, err = strconv.ParseInt(r.strValues[i], 10, 64)
			if err != nil {
				klog.V(10).Infof("ParseInt failed for value %+v in requirement %#v, for 'Gt', 'Lt' operators, the value must be an integer", r.strValues[i], r)
				return false
			}
		}
		return (r.operator == GreaterThan && lsValue > rValue) || (r.operator == LessThan && lsValue < rValue)
	case Match:
		if !ls.Has(r.key) {
			return false
		}
		return !r.hasValue(ls.Get(r.key))
	default:
		return false

	}
}

func (s internalSelector) DeepCopySelector() Selector {
	return s.DeepCopy()
}

func (s internalSelector) DeepCopy() internalSelector {
	if s == nil {
		return nil
	}
	result := make([]Requirement, len(s))
	for i := range s {
		s[i].DeepCopyInto(&result[i])
	}
	return result
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Requirement) DeepCopyInto(out *Requirement) {
	*out = *in
	if in.strValues != nil {
		in, out := &in.strValues, &out.strValues
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// String returns a comma-separated string of all
// the internalSelector Requirements' human-readable strings.
func (s internalSelector) String() string {
	var reqs []string
	for ix := range s {
		reqs = append(reqs, s[ix].String())
	}
	return strings.Join(reqs, ",")
}

// String returns a human-readable string that represents this
// Requirement. If called on an invalid Requirement, an error is
// returned. See NewRequirement for creating a valid Requirement.
func (r *Requirement) String() string {
	var sb strings.Builder
	sb.Grow(
		// length of r.key
		len(r.key) +
			// length of 'r.operator' + 2 spaces for the worst case ('in' and 'notin')
			len(r.operator) + 2 +
			// length of 'r.strValues' slice times. Heuristically 5 chars per word
			+5*len(r.strValues))
	if r.operator == DoesNotExist {
		sb.WriteString("!")
	}
	sb.WriteString(r.key)

	switch r.operator {
	case Equals:
		sb.WriteString("=")
	case DoubleEquals:
		sb.WriteString("==")
	case NotEquals:
		sb.WriteString("!=")
	case In:
		sb.WriteString(" in ")
	case NotIn:
		sb.WriteString(" notin ")
	case GreaterThan:
		sb.WriteString(">")
	case LessThan:
		sb.WriteString("<")
	case Exists, DoesNotExist:
		return sb.String()
	}

	switch r.operator {
	case In, NotIn:
		sb.WriteString("(")
	}
	if len(r.strValues) == 1 {
		sb.WriteString(r.strValues[0])
	} else { // only > 1 since == 0 prohibited by NewRequirement
		// normalizes value order on output, without mutating the in-memory selector representation
		// also avoids normalization when it is not required, and ensures we do not mutate shared data
		sb.WriteString(strings.Join(safeSort(r.strValues), ","))
	}

	switch r.operator {
	case In, NotIn:
		sb.WriteString(")")
	}
	return sb.String()
}

// safeSort sorts input strings without modification
func safeSort(in []string) []string {
	if sort.StringsAreSorted(in) {
		return in
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

// RequiresExactMatch introspects whether a given selector requires a single specific field
// to be set, and if so returns the value it requires.
func (s internalSelector) RequiresExactMatch(label string) (value string, found bool) {
	for ix := range s {
		if s[ix].key == label {
			switch s[ix].operator {
			case Equals, DoubleEquals, In:
				if len(s[ix].strValues) == 1 {
					return s[ix].strValues[0], true
				}
			}
			return "", false
		}
	}
	return "", false
}

// Nothing returns a selector that matches no labels
func Nothing() Selector {
	return nothingSelector{}
}

// Everything returns a selector that matches all labels.
func Everything() Selector {
	return internalSelector{}
}
