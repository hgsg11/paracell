{
  description = "Create isolated per-issue development cells from a project repo";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      revision =
        if self ? shortRev then self.shortRev
        else if self ? dirtyShortRev then self.dirtyShortRev
        else "unknown";
      modified = self.lastModifiedDate or "19700101000000";
      date = "${builtins.substring 0 4 modified}-${builtins.substring 4 2 modified}-${builtins.substring 6 2 modified}";
      version = "0-unstable-${date}-${revision}";
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          paracell = pkgs.buildGoModule {
            pname = "paracell";
            inherit version;

            src = self;
            vendorHash = "sha256-tSLf4m2JlOUq2QqPMYiAzSbTvOzmwfGtjEL69p+j9c8=";

            ldflags = [
              "-s"
              "-w"
              "-X github.com/hgsg11/paracell/internal/app.Version=${version}"
            ];

            nativeBuildInputs = [ pkgs.makeWrapper ];
            postInstall = ''
              wrapProgram $out/bin/paracell \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.git pkgs.tmux ]}
            '';

            meta = {
              description = "Create isolated per-issue development cells from a project repo";
              homepage = "https://github.com/hgsg11/paracell";
              license = pkgs.lib.licenses.mit;
              mainProgram = "paracell";
            };
          };
        in
        {
          inherit paracell;
          default = paracell;
        });

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/paracell";
          meta.description = "Run paracell with git and tmux available";
        };
      });
    };
}
