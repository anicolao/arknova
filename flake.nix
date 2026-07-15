{
  description = "Download assets referenced by a Tabletop Simulator Workshop mod";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          tts-assets = pkgs.writeShellApplication {
            name = "tts-assets";
            runtimeInputs = with pkgs; [ coreutils curl file jq ];
            text = builtins.readFile ./scripts/tts-assets.sh;
          };
          extract-steam-assets = pkgs.writeShellApplication {
            name = "extract-steam-assets";
            runtimeInputs = with pkgs; [ python3 uv ];
            text = ''
              exec uv run --quiet --with UnityPy==1.25.2 \
                python ${./scripts/extract-steam-assets.py} "$@"
            '';
          };
        in {
          inherit extract-steam-assets tts-assets;
          default = tts-assets;
        });

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.tts-assets}/bin/tts-assets";
        };
        extract-steam-assets = {
          type = "app";
          program = "${self.packages.${system}.extract-steam-assets}/bin/extract-steam-assets";
        };
      });

      devShells = forAllSystems (system: {
        default = let pkgs = nixpkgs.legacyPackages.${system}; in pkgs.mkShell {
          packages = with pkgs; [
            bun
            go
            nodejs
            sqlite
          ] ++ (with self.packages.${system}; [ extract-steam-assets tts-assets ]);
        };
      });
    };
}
