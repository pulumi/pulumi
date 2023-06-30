{ nixpkgs
, pulumi-sdk
, pulumi-version
}:

nixpkgs.buildGoModule {
  name = "pulumi-cli";
  src = ./.;
  subPackages = [ "cmd/pulumi" ];
  vendorSha256 = "sha256-CbPuBeNvp8BxBTWqt5yePxKVVfpjZEgsXho8d9C+4C4=";
  buildInputs = [ pulumi-sdk ];

  ldflags = [
    "-X" "github.com/pulumi/pulumi/pkg/v3/version.Version=${pulumi-version}"
  ];

  # don't run tests because they make network requests
  doCheck = false; # TODO: fix

  # Add the 'replace' directive to point ot the local pulumi-sdk.
  # Add it as a patch step rather than a build step for better caching.
  prePatch = ''
    go mod edit -replace github.com/pulumi/pulumi/sdk/v3=${pulumi-sdk.src}
  '';
}
