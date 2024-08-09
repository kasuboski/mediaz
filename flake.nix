{
  description = "A Nix-flake-based Go 1.22 development environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      goVersion = 22; # Change this to update the whole stack

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

        # Override mockgen to fetch the latest version from GitHub
        mockgen = final.buildGoModule rec {
          pname = "mockgen";
          version = "v1.6.0"; # Replace with the latest version you want

          src = final.fetchFromGitHub {
            owner = "golang";
            repo = "mock";
            rev = "v1.6.0"; # Replace with the correct version tag
            sha256 = "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"; # Correct hash
          };

          vendorHash = null; # Set to null if you are not using vendored dependencies or replace with correct hash

          meta = with final.lib; {
            description = "GoMock is a mocking framework for the Go programming language.";
            license = licenses.asl20;
            platforms = platforms.all;
          };
        };
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
            
            # https://github.com/golang/mock
            mockgen
          ];

          shellHook = ''
            alias cobra="cobra-cli"
          '';
        };
      });
    };
}
