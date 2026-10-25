{
  description = "gorxiv: search Europe PMC and download open-access PDFs";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
    in {
      packages = forAll (pkgs: {
        default = pkgs.buildGoModule {
          pname = "gorxiv";
          version = self.shortRev or self.dirtyShortRev or "dev";
          src = self;
          vendorHash = "sha256-xKgqKTn/AiQLWtuQ5NtgADL3Uxlug7T7u4gwlJujjAA=";
          env.CGO_ENABLED = 0;
          ldflags = [ "-s" "-w" "-X github.com/btraven00/gorxiv/cmd.version=${self.shortRev or self.dirtyShortRev or "dev"}" ];
          meta.mainProgram = "gorxiv";
        };
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell { packages = [ pkgs.go pkgs.gopls ]; };
      });
    };
}
