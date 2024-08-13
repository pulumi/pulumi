package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type TestUnmarshalArgs struct {
	Somestring   string
	Someint      int
	DashedName   string
	WithOverride string `args:"override"`
	NestedArgs
}

type NestedArgs struct {
	Somebool bool
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("somestring", "hello")
	v.Set("someint", 42)
	v.Set("dashed-name", "dashed")
	v.Set("override", "this goes into WithOverride")
	v.Set("somebool", true)

	args := UnmarshalArgs[TestUnmarshalArgs](v, "")

	require.Equal(t, "hello", args.Somestring)
	require.Equal(t, 42, args.Someint)
	require.Equal(t, "dashed", args.DashedName)
	require.Equal(t, "this goes into WithOverride", args.WithOverride)
	require.Equal(t, true, args.Somebool)
}

// Test unmarshalling options from a config file.
func TestUnmarshalWithFile(t *testing.T) {
	t.Parallel()

	v := viper.New()

	config := `
global:
  somestring: hello
  someint: 42
  dashed-name: dashed
  override: this goes into WithOverride
  sOmEbOoL: true
somesection:
  somestring: hello from somesection
anothersection:
  somestring: hello from anothersection
`

	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(bytes.NewBufferString(config)))

	// Unmarshal without a section
	args := UnmarshalArgs[TestUnmarshalArgs](v, "")

	require.Equal(t, "hello", args.Somestring)
	require.Equal(t, 42, args.Someint)
	require.Equal(t, "dashed", args.DashedName)
	require.Equal(t, "this goes into WithOverride", args.WithOverride)
	require.Equal(t, true, args.Somebool)

	// Unmarshal with a section
	args = UnmarshalArgs[TestUnmarshalArgs](v, "somesection")
	require.Equal(t, "hello from somesection", args.Somestring)
	require.Equal(t, 42, args.Someint)
	require.Equal(t, "dashed", args.DashedName)
	require.Equal(t, "this goes into WithOverride", args.WithOverride)
	require.Equal(t, true, args.Somebool)
}

type TestBindFlagsArgs struct {
	Somestring   string
	DefaultValue string `argsDefault:"the default value"`
	Usage        string `argsUsage:"this is the usage description"`
	Short        string `argsShort:"S"`
}

// Test that BindFlags sets up the cobra flags correctly.
func TestBindFlags(t *testing.T) {
	t.Parallel()

	v := viper.New()
	cmd := &cobra.Command{}

	BindFlags[TestBindFlagsArgs](v, cmd)
	require.NoError(t, cmd.ParseFlags([]string{}))

	flag := cmd.Flags().Lookup("somestring")
	require.NotNil(t, flag)

	flag = cmd.Flags().Lookup("default-value")
	require.NotNil(t, flag)
	require.Equal(t, "the default value", flag.DefValue)

	flag = cmd.Flags().Lookup("usage")
	require.NotNil(t, flag)
	require.Equal(t, "this is the usage description", flag.Usage)

	flag = cmd.Flags().Lookup("short")
	require.NotNil(t, flag)
	require.Equal(t, "S", flag.Shorthand)
}

type TestUnmarshalWithFileAndFlagsArgs struct {
	One   string
	Two   string
	Three string
	Four  string
	Five  string `argsDefault:"set in default"`
}

// Test priority of data sources for args. Flags > File Section > File Global > Default
func TestUnmarshalWithFileAndFlags(t *testing.T) {
	t.Parallel()

	v := viper.New()

	config := `
global:
  one: set in global
  two: set in global
  four: set in global only
somesection:
  one: set in somesection
  two: set in somesection
  three: set in some section only
`
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(bytes.NewBufferString(config)))
	cmd := &cobra.Command{}
	BindFlags[TestUnmarshalWithFileAndFlagsArgs](v, cmd)
	require.NoError(t, cmd.ParseFlags([]string{"--one", "set in flag"}))

	args := UnmarshalArgs[TestUnmarshalWithFileAndFlagsArgs](v, "somesection")

	// Flag takes the highest priority
	require.Equal(t, "set in flag", args.One)
	// Specific section > global section
	require.Equal(t, "set in somesection", args.Two)
	// Section only
	require.Equal(t, "set in some section only", args.Three)
	// Global only
	require.Equal(t, "set in global only", args.Four)
	// Default
	require.Equal(t, "set in default", args.Five)
}
