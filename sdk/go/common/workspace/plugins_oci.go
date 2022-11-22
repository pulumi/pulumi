package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/blang/semver"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

var ociPackageVersionAnnotation string = "com.pulumi.package.version"

var ociPluginAnnotation string = "com.pulumi.plugin"
var ociPluginOsAnnotation string = "com.pulumi.plugin.os"
var ociPluginArchAnnotation string = "com.pulumi.plugin.arch"

var defaultRegistry string = "localhost:5000"
var defaultRegistryPlainHTTP bool = true

type ociSource struct {
	name string
	kind PluginKind

	repo registry.Repository

	repoName string
}

func newOCISource(name string, kind PluginKind) (ociSource, error) {
	reg, err := remote.NewRegistry(defaultRegistry)
	if err != nil {
		return ociSource{}, err
	}
	reg.PlainHTTP = defaultRegistryPlainHTTP

	var ns, pkg string
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		ns, pkg = parts[0], parts[1]
	} else {
		ns, pkg = "pulumi", name
	}
	repoName := fmt.Sprintf("%s/%s-%s", ns, kind, pkg)

	repo, err := reg.Repository(context.Background(), repoName)

	// fmt.Printf("🦖🦖🦖 using oci plugin source repository http://%v/%v\n", defaultRegistry, repoName)
	return ociSource{
		name:     name,
		kind:     kind,
		repo:     repo,
		repoName: repoName,
	}, nil
}

// Download fetches an io.ReadCloser for this plugin and also returns the size of the response (if known).
func (s *ociSource) Download(version semver.Version, opSy string, arch string, getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error)) (io.ReadCloser, int64, error) {
	tag := "v" + version.String()
	artifacts, err := s.listArtifacts(tag)

	if err != nil {
		return nil, 0, err
	}

	return s.downloadPlugin(artifacts, opSy, arch)
}

// GetLatestVersion tries to find the latest version for this plugin. This is currently only supported for
// plugins we can get from github releases.
func (s *ociSource) GetLatestVersion(getHTTPResponse func(*http.Request) (io.ReadCloser, int64, error)) (*semver.Version, error) {
	ctx := context.Background()

	tag := "latest"
	// fmt.Printf("🦖🦖🦖 querying OCI for reference %v:%v\n", s.repoName, tag)
	desc, resp, err := s.repo.FetchReference(ctx, tag)
	defer resp.Close()
	if err != nil {
		return nil, err
	}

	content, err := content.ReadAll(resp, desc)
	if err != nil {
		return nil, err
	}

	var manifest ocispec.Descriptor
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}

	// fmted, _ := json.MarshalIndent(manifest, "👀 ", "  ")
	// fmt.Printf("🦖🦖🦖 found OCI manifest:\n👀 %v\n", string(fmted))

	annotation, has := manifest.Annotations[ociPackageVersionAnnotation]
	if !has {
		return nil, fmt.Errorf("missing version annotation %q on tag %q", ociPackageVersionAnnotation, tag)
	}

	version, err := semver.Parse(annotation)
	if err != nil {
		return nil, fmt.Errorf("unable to parse version: %w", err)
	}

	// fmt.Printf("🦖🦖🦖 found latest version via annotation: %v\n", version.String())
	return &version, nil
}

func (s *ociSource) listArtifacts(tag string) ([]ocispec.Descriptor, error) {
	ctx := context.Background()

	ref, err := s.repo.Resolve(ctx, tag)
	if err != nil {
		return nil, err
	}

	items, err := content.Successors(ctx, s.repo, ref)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (s *ociSource) downloadPlugin(artifacts []ocispec.Descriptor, os string, arch string) (io.ReadCloser, int64, error) {
	// fmt.Printf("🦖🦖🦖 iterating over %v artifacts to find %v-%v plugin\n", len(artifacts), os, arch)
	for _, a := range artifacts {
		if val, has := a.Annotations[ociPluginAnnotation]; !has || val != "true" {
			continue
		}
		if val, has := a.Annotations[ociPluginOsAnnotation]; !has || val != os {
			continue
		}
		if val, has := a.Annotations[ociPluginArchAnnotation]; !has || val != arch {
			continue
		}

		// fmted, _ := json.MarshalIndent(a, "👀 ", "  ")
		// fmt.Printf("🦖🦖🦖 downloading plugin artifact:\n👀 %v\n", string(fmted))
		resp, err := s.repo.Blobs().Fetch(context.Background(), a)
		// fmt.Println("🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀")
		return resp, a.Size, err
	}

	return nil, 0, fmt.Errorf("plugin not found")
}
