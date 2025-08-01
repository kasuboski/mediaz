{
  description = "A Nix-flake-based Go 1.22 development environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      goVersion = 24; # Change this to update the whole stack

      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forEachSupportedSystem = f: nixpkgs.lib.genAttrs supportedSystems (system: f {
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ self.overlays.default ];
        };
      });
    in
    {
      overlays.default = final: prev: {
        go = final."go_1_${toString goVersion}";
      };

      devShells = forEachSupportedSystem ({ pkgs }: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            # go (version is specified by overlay)
            go

            # goimports, godoc, etc.
            gotools

            # https://github.com/golangci/golangci-lint
            golangci-lint

            # https://github.com/spf13/cobra
            cobra-cli

            # https://github.com/uber-go/mock
            mockgen

            # nix formatter
            nixpkgs-fmt

            # https://github.com/go-jet/jet
            go-jet

            # https://github.com/sqlite/sqlite
            sqlite

            nodejs
          ];

          shellHook = ''
            alias cobra="cobra-cli"
          '';
        };
      });
    };
}
