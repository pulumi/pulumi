// Copyright 2016 Marapongo, Inc. All rights reserved.

package pack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageURLStringParse(t *testing.T) {
	{
		s := "simple"
		p, err := PackageURLString(s).Parse()
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
		s := string("simple#" + DefaultPackageURLVersion)
		p, err := PackageURLString(s).Parse()
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
		s := "simple#1.0.6"
		p, err := PackageURLString(s).Parse()
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
		s := "simple#>=1.0.6"
		p, err := PackageURLString(s).Parse()
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
		s := "simple#6f99088"
		p, err := PackageURLString(s).Parse()
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
		s := "simple#83030685c3b8a3dbe96bd10ab055f029667a96b0"
		p, err := PackageURLString(s).Parse()
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
		p, err := PackageURLString(s).Parse()
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
		p, err := PackageURLString(s).Parse()
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
		s := "_/_/_/_/a0/c0Mpl3x_"
		p, err := PackageURLString(s).Parse()
		assert.Nil(t, err)
		assert.Equal(t, s, p.String())
		assert.Equal(t, "", p.Proto)
		assert.Equal(t, "", p.Base)
		assert.Equal(t, "_/_/_/_/a0/c0Mpl3x_", string(p.Name))
		assert.Equal(t, "", string(p.Version))
		p = p.Defaults()
		assert.Equal(t, DefaultPackageURLProto, p.Proto)
		assert.Equal(t, DefaultPackageURLBase, p.Base)
		assert.Equal(t, "_/_/_/_/a0/c0Mpl3x_", string(p.Name))
		assert.Equal(t, DefaultPackageURLVersion, p.Version)
	}
	{
		s := "github.com/ns1/ns2/ns3/ns4/complex"
		p, err := PackageURLString(s).Parse()
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
		s := "git://github.com/ns1/ns2/ns3/ns4/complex"
		p, err := PackageURLString(s).Parse()
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
		s := "git://github.com/ns1/ns2/ns3/ns4/complex#1.0.6"
		p, err := PackageURLString(s).Parse()
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
		s := "git://github.com/ns1/ns2/ns3/ns4/complex#>=1.0.6"
		p, err := PackageURLString(s).Parse()
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
		s := "git://github.com/ns1/ns2/ns3/ns4/complex#6f99088"
		p, err := PackageURLString(s).Parse()
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
		s := "git://github.com/ns1/ns2/ns3/ns4/complex#83030685c3b8a3dbe96bd10ab055f029667a96b0"
		p, err := PackageURLString(s).Parse()
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
