package introspect

import "fmt"

func NewAnnotator(resource any) Annotator {
	return Annotator{
		Descriptions: map[string]string{},
		Defaults:     map[string]any{},
		matcher:      NewFieldMatcher(resource),
	}
}

// Implements the Annotator interface as defined in resource/resource.go
type Annotator struct {
	Descriptions map[string]string
	Defaults     map[string]any

	matcher FieldMatcher
}

func (a *Annotator) mustGetField(i any) FieldTag {
	field, err, ok := a.matcher.GetField(i)
	if !ok {
		panic("Could not annotate field: could not find field")
	}
	if err != nil {
		panic(fmt.Sprintf("Could not parse field tags: %s", err.Error()))
	}
	return field
}

func (a *Annotator) Describe(i any, description string) {
	if a.matcher.value.Interface() == i {
		a.Descriptions[""] = description
		return
	}
	field := a.mustGetField(i)
	a.Descriptions[field.Name] = description
}

// Annotate a a struct field with a default value. The default value must be a primitive
// type in the pulumi type system.
func (a *Annotator) SetDefault(i any, defaultValue any) {
	field := a.mustGetField(i)
	a.Defaults[field.Name] = defaultValue
}
