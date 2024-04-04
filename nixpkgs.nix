let
  nixpkgs = fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/bd22babc47eb6244acc46880aac65c8c82678051.tar.gz";
  };
in
  import nixpkgs
