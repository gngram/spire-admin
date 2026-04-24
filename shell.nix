{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  buildInputs = [
    pkgs.go
    pkgs.gopls
    pkgs.go-tools
    pkgs.delve
    pkgs.golangci-lint
    pkgs.cmake
  ];

  GO111MODULE = "on";

  shellHook = ''
    export GOPATH="$PWD/.gopath"
    echo "Go dev shell activated"
  '';
}

