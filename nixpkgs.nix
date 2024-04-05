let
  nixpkgs = fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/bd22babc47eb6244acc46880aac65c8c82678051.tar.gz";
  };

  overlay = self: super: {
    bazel_7 = super.bazel_7.override (previous: previous // {
      python3 = self.python3WithPackages;
    });

    python3WithPackages = super.python3.withPackages (ps: with ps; [
      build
      pip
    ]);
  };
in
  args@{ overlays ? [], ... }:
    import nixpkgs (args // {
      overlays = [overlay] ++ overlays;
    })
