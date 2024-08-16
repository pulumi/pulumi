package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestGetScopes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		cmd  func() *cobra.Command
		want []string
	}{
		{
			cmd: func() *cobra.Command {
				return &cobra.Command{Use: "pulumi"}
			},
			want: []string{"pulumi"},
		},
		{
			cmd: func() *cobra.Command {
				pulumiCmd := &cobra.Command{Use: "pulumi"}
				childCmd := &cobra.Command{Use: "child"}

				pulumiCmd.AddCommand(childCmd)

				return childCmd
			},
			want: []string{"child", "pulumi"},
		},
		{
			cmd: func() *cobra.Command {
				pulumiCmd := &cobra.Command{Use: "pulumi"}
				childCmd := &cobra.Command{Use: "child"}
				grandchildCmd := &cobra.Command{Use: "grandchild"}

				pulumiCmd.AddCommand(childCmd)
				childCmd.AddCommand(grandchildCmd)

				return grandchildCmd
			},
			want: []string{"child:grandchild", "child", "pulumi"},
		},
		{
			cmd: func() *cobra.Command {
				pulumiCmd := &cobra.Command{Use: "pulumi"}
				childCmd := &cobra.Command{Use: "child"}
				grandchildCmd := &cobra.Command{Use: "grandchild"}
				greatGrandchildCmd := &cobra.Command{Use: "great-grandchild"}

				pulumiCmd.AddCommand(childCmd)
				childCmd.AddCommand(grandchildCmd)
				grandchildCmd.AddCommand(greatGrandchildCmd)

				return greatGrandchildCmd
			},
			want: []string{"child:grandchild:great-grandchild", "child:grandchild", "child", "pulumi"},
		},
	}

	for _, c := range cases {
		got := getScopes(c.cmd())
		require.Equal(t, c.want, got)
	}
}

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

	cmd := &cobra.Command{Use: "cmd"}

	v := viper.New()
	v.Set("cmd.somestring", "hello")
	v.Set("cmd.someint", 42)
	v.Set("cmd.dashed-name", "dashed")
	v.Set("cmd.override", "this goes into WithOverride")
	v.Set("cmd.somebool", true)

	args := UnmarshalArgs[TestUnmarshalArgs](v, cmd)

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

	globalCmd := &cobra.Command{Use: "global"}
	child1Cmd := &cobra.Command{Use: "child1"}
	child2Cmd := &cobra.Command{Use: "child2"}

	globalCmd.AddCommand(child1Cmd)
	globalCmd.AddCommand(child2Cmd)

	config := `
global:
  somestring: hello
  someint: 42
  dashed-name: dashed
  override: this goes into WithOverride
  sOmEbOoL: true
child1:
  somestring: hello from child1
child2:
  somestring: hello from child2
`

	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(bytes.NewBufferString(config)))

	// Unmarshal the global command
	args := UnmarshalArgs[TestUnmarshalArgs](v, globalCmd)

	require.Equal(t, "hello", args.Somestring)
	require.Equal(t, 42, args.Someint)
	require.Equal(t, "dashed", args.DashedName)
	require.Equal(t, "this goes into WithOverride", args.WithOverride)
	require.Equal(t, true, args.Somebool)

	// Unmarshal with a section
	args = UnmarshalArgs[TestUnmarshalArgs](v, child1Cmd)
	require.Equal(t, "hello from child1", args.Somestring)
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
child1:
  one: set in child1
  two: set in child1
  three: set in child1 only
`
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(bytes.NewBufferString(config)))

	globalCmd := &cobra.Command{Use: "global"}
	child1Cmd := &cobra.Command{Use: "child1"}

	globalCmd.AddCommand(child1Cmd)

	BindFlags[TestUnmarshalWithFileAndFlagsArgs](v, child1Cmd)
	require.NoError(t, child1Cmd.ParseFlags([]string{"--one", "set in flag"}))

	args := UnmarshalArgs[TestUnmarshalWithFileAndFlagsArgs](v, child1Cmd)

	// Flag takes the highest priority
	require.Equal(t, "set in flag", args.One)
	// Child section > parent section
	require.Equal(t, "set in child1", args.Two)
	// Child section only
	require.Equal(t, "set in child1 only", args.Three)
	// Parent section only
	require.Equal(t, "set in global only", args.Four)
	// Default
	require.Equal(t, "set in default", args.Five)
}
