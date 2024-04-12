let
  base =
    fetchTarball "https://github.com/NixOS/nixpkgs/archive/f59f29659e1db1130f87936ef9f2a11a71b3a99c.zip";

  overlay = self: super: {

  };

in
  args@{ overlays ? [], ... }:
    import base (args // {
      overlays = [overlay] ++ overlays;
    })
