{
  description = "spire_admin Go Development Environment";

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
            nativeBuildInputs = with pkgs; [
              pkg-config
              go
              gotools
              gofumpt
              spire
            ];

            buildInputs = with pkgs; [
              gcc
              pkg-config
              libGL
              libGLU
              # Fyne / GLFW Dependencies
              libx11.dev
              libxcursor
              libxrandr
              libxinerama
              libxi.dev
              libxxf86vm
              libglvnd.dev # OpenGL headers
              glfw
              freetype
              dbus
              openssl
            ];
            GO111MODULE = "on";

            shellHook = ''
              alias run="go run ."
              alias build-desktop-app="go build ./apps/spire-admin-desktop"
              alias build-web-app="go build ./apps/spire-admin-web"
              alias fmt="find . -path ./.gopath -prune -o -name '*.go' -exec gofumpt -l -w {} +"
              export GOPATH="$PWD/.gopath"
              echo "Go dev shell activated (via Flake)"

              if [ ! -f go.mod ]; then
                echo "Initializing Go module 'github.com/gngram/spire_admin'..."
                go mod init github.com/gngram/spire_admin
              fi

              echo "Tidying up and downloading missing Go modules..."
              go mod tidy
              echo "================================================"
              echo "                 SPIRE ADMIN"
              echo "================================================"
              echo "Shell commands:"
              echo "build-desktop-app:   build desktop application"
              echo "build-web-app:       build web application"
              echo "================================================"
            '';
          };
        });
    };
}
