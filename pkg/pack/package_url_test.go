// Copyright 2017 Pulumi, Inc. All rights reserved.

package pack

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/coconut/pkg/tokens"
)

func TestPackageURLStringParse(t *testing.T) {
	{
		s := "*"
		pkg := tokens.PackageName("simple")
		p, err := PackageURLString(s).Parse(pkg)
		assert.Nil(t, err)
		assert.Equal(t, pkg.String(), p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		s := "simple"
		p, err := PackageURLString(s).Parse(tokens.PackageName(s))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		b := "simple"
		s := b + "#" + string(DefaultPackageURLVersion)
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		b := "simple"
		s := b + "#1.0.6"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec("1.0.6"), p.Version)
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec("1.0.6"), p.Version)
	}
	{
		b := "simple"
		s := b + "#>=1.0.6"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec(">=1.0.6"), p.Version)
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec(">=1.0.6"), p.Version)
	}
	{
		b := "simple"
		s := b + "#6f99088"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec("6f99088"), p.Version)
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec("6f99088"), p.Version)
	}
	{
		b := "simple"
		s := b + "#83030685c3b8a3dbe96bd10ab055f029667a96b0"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec("83030685c3b8a3dbe96bd10ab055f029667a96b0"), p.Version)
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "simple", string(p.Name))
		assert.Equal(t, VersionSpec("83030685c3b8a3dbe96bd10ab055f029667a96b0"), p.Version)
	}
	{
		s := "namespace/complex"
		p, err := PackageURLString(s).Parse(tokens.PackageName(s))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "namespace/complex", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "namespace/complex", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		s := "ns1/ns2/ns3/ns4/complex"
		p, err := PackageURLString(s).Parse(tokens.PackageName(s))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		s := "_/_/_/_a-b/a0/c0Mpl3x_"
		p, err := PackageURLString(s).Parse(tokens.PackageName(s))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "_/_/_/_a-b/a0/c0Mpl3x_", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "_/_/_/_a-b/a0/c0Mpl3x_", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		b := "ns1/ns2/ns3/ns4/complex"
		s := "github.com/" + b
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		b := "ns1/ns2/ns3/ns4/complex"
		s := "git://github.com/" + b
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		b := "ns1/ns2/ns3/ns4/complex"
		s := "git://github.com/" + b + "#1.0.6"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, VersionSpec("1.0.6"), p.Version)
		p = p.Defaults()
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, VersionSpec("1.0.6"), p.Version)
	}
	{
		b := "ns1/ns2/ns3/ns4/complex"
		s := "git://github.com/" + b + "#>=1.0.6"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, VersionSpec(">=1.0.6"), p.Version)
		p = p.Defaults()
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, ">=1.0.6", string(p.Version))
	}
	{
		b := "ns1/ns2/ns3/ns4/complex"
		s := "git://github.com/" + b + "#6f99088"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, VersionSpec("6f99088"), p.Version)
		p = p.Defaults()
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, "6f99088", string(p.Version))
	}
	{
		b := "ns1/ns2/ns3/ns4/complex"
		s := "git://github.com/" + b + "#83030685c3b8a3dbe96bd10ab055f029667a96b0"
		p, err := PackageURLString(s).Parse(tokens.PackageName(b))
		assert.Nil(t, err)
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, VersionSpec("83030685c3b8a3dbe96bd10ab055f029667a96b0"), p.Version)
		p = p.Defaults()
		assert.Equal(t, "git://", p.Proto)
		assert.Equal(t, "github.com/", p.Base)
		assert.Equal(t, "ns1/ns2/ns3/ns4/complex", string(p.Name))
		assert.Equal(t, "83030685c3b8a3dbe96bd10ab055f029667a96b0", string(p.Version))
	}
}
