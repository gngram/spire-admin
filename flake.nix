{
  description = "Spidar Go Development Environment";

  inputs = {
    # Define your nixpkgs version here. 
    # e.g., github:NixOS/nixpkgs/nixos-unstable or path:/work/repositories/gngram/ghaf
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in {
      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              nodejs
              go
              gopls
              go-tools
              delve
              golangci-lint
              cmake
            ];

            GO111MODULE = "on";

            shellHook = ''
              alias gemini="npx @google/gemini-cli"
              export GOPATH="$PWD/.gopath"
              echo "Go dev shell activated (via Flake)"
              echo "Tidying up and downloading missing Go modules..."
              go mod tidy
            '';
          };
        });
    };
}
