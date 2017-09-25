package option

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/rliebz/tusk/config/when"
)

// Arg represents a command-line argument.
// An Arg is passed by position and is always required.
type Arg struct {
	Type  string
	Usage string

	// Computed members not specified in yaml file
	Value string `yaml:"-"`
}

// Option represents an abstract command-line option.
// Options are passed by flags on the command line.
type Option struct {
	Short    string
	Type     string
	Usage    string
	Private  bool
	Required bool

	// Used to determine value
	Environment   string
	DefaultValues valueList `yaml:"default"`

	// Computed members not specified in yaml file
	Name   string            `yaml:"-"`
	Passed string            `yaml:"-"`
	Vars   map[string]string `yaml:"-"`
}

// Dependencies returns a list of options that are required explicitly.
// This does not include interpolations.
func (o *Option) Dependencies() []string {
	var options []string

	for _, value := range o.DefaultValues {
		options = append(options, value.When.Dependencies()...)
	}

	return options
}

// UnmarshalYAML ensures that the option definition is valid.
func (o *Option) UnmarshalYAML(unmarshal func(interface{}) error) error {

	type optionType Option // Use new type to avoid recursion
	if err := unmarshal((*optionType)(o)); err != nil {
		return err
	}

	if len(o.Short) > 1 {
		return fmt.Errorf(
			`option short name "%s" cannot exceed one character`,
			o.Short,
		)
	}

	if o.Private && o.Required {
		return errors.New("option cannot be both private and required")
	}

	if o.Private && o.Environment != "" {
		return fmt.Errorf(
			`environment variable "%s" defined for private option`,
			o.Environment,
		)
	}

	if o.Required && len(o.DefaultValues) > 0 {
		return errors.New("default value defined for required option")
	}

	return nil
}

// Value determines an option's final value based on all configuration.
//
// For non-private variables, the order of priority is:
//   1. Parameter that was passed
//   2. Environment variable set
//   3. The first item in the default value list with a valid when clause
func (o *Option) Value() (string, error) {

	if o == nil {
		return "", nil
	}

	if !o.Private {
		if o.Passed != "" {
			return o.Passed, nil
		}

		envValue := os.Getenv(o.Environment)
		if envValue != "" {
			return envValue, nil
		}
	}

	if o.Required {
		return "", fmt.Errorf("no value passed for required option: %s", o.Name)
	}

	for _, candidate := range o.DefaultValues {
		if err := candidate.When.Validate(o.Vars); err != nil {
			if !when.IsFailedCondition(err) {
				return "", err
			}
			continue
		}

		value, err := candidate.commandValueOrDefault()
		if err != nil {
			return "", errors.Wrapf(err, "could not compute value for option: %s", o.Name)
		}

		return value, nil
	}

	return "", nil
}

type value struct {
	When    when.When
	Command string
	Value   string
}

// commandValueOrDefault validates a content definition, then gets the value.
func (v *value) commandValueOrDefault() (string, error) {

	if v.Command != "" {
		out, err := exec.Command("sh", "-c", v.Command).Output() // nolint: gas
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(string(out)), nil
	}

	return v.Value, nil
}

// UnmarshalYAML allows plain strings to represent a full struct. The value of
// the string is used as the Default field.
func (v *value) UnmarshalYAML(unmarshal func(interface{}) error) error {

	var err error

	var valueString string
	if err = unmarshal(&valueString); err == nil {
		*v = value{Value: valueString}
		return nil
	}

	type valueType value // Use new type to avoid recursion
	if err = unmarshal((*valueType)(v)); err == nil {

		if v.Value != "" && v.Command != "" {
			return fmt.Errorf(
				"value (%s) and command (%s) are both defined",
				v.Value, v.Command,
			)
		}

		return nil
	}

	return err
}

type valueList []value

// UnmarshalYAML allows single items to be used as lists.
func (vl *valueList) UnmarshalYAML(unmarshal func(interface{}) error) error {

	var err error

	var valueSlice []value
	if err = unmarshal(&valueSlice); err == nil {
		*vl = valueSlice
		return nil
	}

	var valueItem value
	if err = unmarshal(&valueItem); err == nil {
		*vl = valueList{valueItem}
		return nil
	}

	return err
}
