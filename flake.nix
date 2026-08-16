{
  description = "Ark Nova digital tabletop development and deployment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    bun2nix = {
      url = "github:nix-community/bun2nix?ref=2.1.2";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, bun2nix }:
    let
      supportedSystems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ bun2nix.overlays.default ];
          };
          lib = pkgs.lib;
          sourceRevision = self.rev or "dirty";
          contentManifestExists = builtins.pathExists ./content/synthetic/manifest.json;
          contentVersion = if contentManifestExists
            then (builtins.fromJSON (builtins.readFile ./content/synthetic/manifest.json)).version
            else "none";
          web-client = pkgs.stdenvNoCC.mkDerivation {
            pname = "arknova-web";
            version = "0";
            src = ./web;
            nativeBuildInputs = [ pkgs.bun2nix.hook ];
            bunDeps = pkgs.bun2nix.fetchBunDeps { bunNix = ./web/bun.nix; };
            bunInstallFlags = [ "--linker=hoisted" ]
              ++ lib.optionals pkgs.stdenv.hostPlatform.isDarwin [ "--backend=copyfile" ];
            buildPhase = ''
              runHook preBuild
              bun run build
              runHook postBuild
            '';
            installPhase = ''
              runHook preInstall
              mkdir -p $out
              cp -R build/. $out/
              runHook postInstall
            '';
          };
          arknova-server = pkgs.buildGoModule {
            pname = "arknova-server";
            version = "0";
            src = lib.fileset.toSource {
              root = ./.;
              fileset = lib.fileset.unions [ ./go.mod ./go.sum ./cmd ./internal ];
            };
            subPackages = [ "cmd/arknova" ];
            env.CGO_ENABLED = "0";
            ldflags = [ "-s" "-w" ];
            vendorHash = "sha256-qg3uqojotJz9NnqcBxaj3Pf9mm7XhikIMmlRCmfE2yo=";
          };
          buildInfo = pkgs.writeText "build.json" (builtins.toJSON {
            repository = "github.com/anicolao/arknova";
            commit = sourceRevision;
            goVersion = pkgs.go.version;
            bunVersion = pkgs.bun.version;
            inherit contentVersion;
            artifactFormatVersion = 1;
          });
          arknova-release = pkgs.runCommand "arknova-release-${builtins.substring 0 12 sourceRevision}" { } (''
            mkdir -p $out/bin $out/web
            cp ${arknova-server}/bin/arknova $out/bin/arknova
            cp -R ${web-client}/. $out/web/
            cp ${buildInfo} $out/build.json
          '' + lib.optionalString contentManifestExists ''
            mkdir -p $out/content/synthetic
            cp -R ${./content/synthetic}/. $out/content/synthetic/
          '' + ''
            chmod 0555 $out/bin/arknova
            find $out/web -type d -exec chmod 0555 {} +
            find $out/web -type f -exec chmod 0444 {} +
            chmod 0444 $out/build.json
          '');
          package-preview = pkgs.writeShellApplication {
            name = "package-arknova-preview";
            runtimeInputs = with pkgs; [ coreutils file findutils gawk gnugrep gnutar gzip jq ];
            text = builtins.readFile ./scripts/package-preview.sh;
          };
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
          inherit arknova-release arknova-server extract-steam-assets package-preview tts-assets web-client;
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
        package-preview = {
          type = "app";
          program = "${self.packages.${system}.package-preview}/bin/package-arknova-preview";
        };
      });

      devShells = forAllSystems (system: {
        default = let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ bun2nix.overlays.default ];
          };
        in pkgs.mkShell {
          packages = with pkgs; [
            bun
            coreutils
            file
            gnutar
            gzip
            jq
            go
            nodejs
            sqlite
          ] ++ [ pkgs.bun2nix ]
            ++ (with self.packages.${system}; [ extract-steam-assets package-preview tts-assets ]);
        };
      });
    };
}
