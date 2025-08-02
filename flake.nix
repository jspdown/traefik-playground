{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { systems, nixpkgs, ... }@inputs:
    let
      eachSystem = f: nixpkgs.lib.genAttrs (import systems) (system:
        let
          pkgs = import nixpkgs {
            system = system;
          };
        in f pkgs
      );
    in
    {
      devShells = eachSystem (pkgs: {
        default = pkgs.mkShell {
          buildInputs = [
            pkgs.gnumake
            pkgs.go
            pkgs.golangci-lint
            pkgs.nodejs_22
            pkgs.yarn-berry
          ];
        };
      });
    };
}
