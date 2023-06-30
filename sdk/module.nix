{ nixpkgs
}:

nixpkgs.buildGoModule {
  name = "pulumi-sdk";
  src = ./.;
  subPackages = "go/pulumi";
  vendorSha256 = "sha256-S8eb2V7ZHhQ0xas+88lwxjL50+22dbyJ0aM60dAtb5k=";
}
