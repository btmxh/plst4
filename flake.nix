{
  description = "plst4 development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
  };

  outputs = { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = f:
        nixpkgs.lib.genAttrs systems (system:
          f (import nixpkgs { inherit system; })
        );
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            air
            delve
            ffmpeg-headless
            go_1_23
            gopls
            gotools
            migrate
            nodejs_22
            pnpm
            postgresql_16
          ];

          shellHook = ''
            export GOTOOLCHAIN=local

            echo "plst4 dev shell"
            echo "  pnpm install"
            echo "  migrate -database \"$DATABASE_URL\" -path db/migrations up"
            echo "  pnpm dev"
          '';
        };
      });
    };
}
