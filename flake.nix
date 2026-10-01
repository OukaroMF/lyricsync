{
  description = "Waybar lyrics for go-musicfox with word-by-word timing, translations and romanization";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      version = "0.1.0";

      mkLyricsync = pkgs:
        pkgs.buildGoModule {
          pname = "lyricsync";
          inherit version;

          src = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = pkgs.lib.fileset.unions [
              ./go.mod
              ./go.sum
              ./cmd
              ./internal
            ];
          };

          vendorHash = "sha256-WUTGAYigUjuZLHO1YpVhFSWpvULDZfGMfOXZQqVYAfs=";
          subPackages = [ "cmd/lyricsync" ];

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          postInstall = ''
            install -Dm644 ${./LICENSE} "$out/share/licenses/lyricsync/LICENSE"
          '';

          meta = {
            description = "Display synced NetEase lyrics in Waybar";
            homepage = "https://github.com/oukaromf/lyricsync";
            license = pkgs.lib.licenses.gpl3Plus;
            mainProgram = "lyricsync";
            platforms = pkgs.lib.platforms.linux;
          };
        };
    in {
      packages = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in {
          lyricsync = mkLyricsync pkgs;
          default = self.packages.${system}.lyricsync;
        });

      apps = forAllSystems (system: {
        lyricsync = {
          type = "app";
          program = "${self.packages.${system}.lyricsync}/bin/lyricsync";
          meta.description = "Run lyricsync";
        };
        default = self.apps.${system}.lyricsync;
      });

      checks = forAllSystems (system: {
        inherit (self.packages.${system}) lyricsync;
      });

      devShells = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system};
        in {
          default = pkgs.mkShell {
            packages = [ pkgs.go ];
          };
        });

      overlays.default = final: _prev: {
        lyricsync = mkLyricsync final;
      };
    };
}
