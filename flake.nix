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
            nativeBuildInputs = with pkgs; [
              pkg-config
              go
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
            ];
            GO111MODULE = "on";

            shellHook = ''
              alias run="go run ."
              alias build="go build -o spidar"
              export GOPATH="$PWD/.gopath"
              echo "Go dev shell activated (via Flake)"

              if [ ! -f go.mod ]; then
                echo "Initializing Go module 'github.com/gngram/spidar'..."
                go mod init github.com/gngram/spidar
              fi

              echo "Tidying up and downloading missing Go modules..."
              go mod tidy
            '';
          };
        });
    };
}
