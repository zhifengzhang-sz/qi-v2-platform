# The platform's tools, the same on every machine (docs/process.md): `nix develop`, then `make install` and the
# targets of the Makefile. Everything from outside is an input pinned by flake.lock: qi-v2's one nixpkgs, its pure
# shell and its Go preset; the kit, which writes the agents' files from agents/; the services stack, the package
# `stack` of qi-v2-services, run by make stack-up and make stack-down on the machine's Docker engine, and its test
# stack, by make stack-test-up and make stack-test-down; market's Go form, the package market-go of qi-v2-contracts,
# which the Go workspace (go.work) takes, and market's vectors, which the library's tests read; @qi/chart, the
# packages qi-chart and lightweight-charts of qi-v2-visualization, from which the charting consumer's page is built
# (the package chart-page); the platform's formal definition, the package qi-v2-platform-definition of the project of
# that name, its Lean as source, which the shell names (QI_DEFINITION). Every input is a public repository, read with
# no login. It gives a Nix taker the K-line
# data sets of datasets/ (the product datasets): the package datasets, which installs a version's files into the Nix
# store from GHCR, and lib.dataset, which gives a package each file by its hash.
{
  description = "qi-v2-platform: the streaming architecture and the operations that run on the stream; its tools";

  # The common flakes, in the public qi-v2-nix (its README.md), pinned by this lock: qi-v2, one nixpkgs for every
  # project (this flake's), the pure shell (mkPureShell) and its presets; the kit, which make install builds from
  # this lock's pin and links at dev/agents
  inputs.qi-v2.url = "github:zhifengzhang-sz/qi-v2-nix?dir=qi-v2";
  inputs.nixpkgs.follows = "qi-v2/nixpkgs";
  inputs.kit.url = "github:zhifengzhang-sz/qi-v2-nix?dir=kit";
  inputs.kit.inputs.qi-v2.follows = "qi-v2";
  # The services stack, published by qi-v2-services at its tag: its package `stack` (make stack-up, stack-down) and its
  # test stack (make stack-test-up, stack-test-down)
  inputs.services.url = "github:zhifengzhang-sz/qi-v2-services?ref=stack-0.3.0";
  inputs.services.inputs.qi-v2.follows = "qi-v2";
  # The market data contract, published by qi-v2-contracts at its tag: its package market-go, market's Go form as
  # source, which replaces the module every go.mod of the core and its additions requires at v0.0.0 (docs/impl/engineering/go.toolchain.md, 2);
  # its vectors, in the input's source, to which the library holds the base of its K-lines (lib/README.md)
  inputs.contracts.url = "github:zhifengzhang-sz/qi-v2-contracts?ref=market-1.1.0";
  inputs.contracts.inputs.qi-v2.follows = "qi-v2";
  inputs.contracts.inputs.kit.follows = "kit";
  # @qi/chart, the chart library, published by qi-v2-visualization at its release's tag
  # (docs/designs/design-chart-consumer-2026-11-09.md, What the platform takes): its packages qi-chart (@qi/chart and
  # what it takes, as a node_modules, with its tv engine's script) and lightweight-charts (Lightweight Charts'
  # standalone build and its licence), from which chart-page is built. Each of its inputs at the commit of this flake's
  # same input follows this flake's: qi-v2 and kit; its contracts (chanlun-3.1.0) and market (market-1.1.0), both at the
  # commit this flake's contracts pins; and so nixpkgs. Its qi-v2-ts, @qi/base and @qi/core, is a node of its own.
  inputs.visualization.url = "github:zhifengzhang-sz/qi-v2-visualization?ref=qi-chart-3.0.0";
  inputs.visualization.inputs.qi-v2.follows = "qi-v2";
  inputs.visualization.inputs.kit.follows = "kit";
  inputs.visualization.inputs.contracts.follows = "contracts";
  inputs.visualization.inputs.market.follows = "contracts";

  # The platform's formal definition, published by qi-v2-platform-definition at its release's tag: its package
  # qi-v2-platform-definition, the Lake package of its Lean as source (the base, Kafka's topic, the four components and
  # their wiring), which the shell names (QI_DEFINITION). Its qi-v2 and kit follow this flake's, and so its nixpkgs.
  inputs.definition.url = "github:zhifengzhang-sz/qi-v2-platform-definition?ref=qi-v2-platform-definition-0.1.0";
  inputs.definition.inputs.qi-v2.follows = "qi-v2";
  inputs.definition.inputs.kit.follows = "kit";

  outputs = { self, nixpkgs, qi-v2, kit, services, contracts, visualization, definition }:
    let
      systems = [ "x86_64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f (import nixpkgs { inherit system; }));
    in {
      devShells = forAll (pkgs: {
        default = qi-v2.lib.mkPureShell pkgs {
          name = "platform";
          # Go and gopls, their caches under ~/.cache/platform/; GOTOOLCHAIN=local: the flake's go, never another.
          # Every program is Go: the TypeScript apps were retired (docs/decisions.md, 2026-10-28). The Go workspace:
          # QI_MARKET_GO, market-go's store path, from which make go.work writes go.work at the root (never committed),
          # written on entry too; QI_MARKET_VECTORS, market's vectors in the input's source (market/vectors), which
          # lib/massive's tests read; QI_DEFINITION, the definition's package, its Lean's files at its root
          uses = [
            (qi-v2.lib.go pkgs)
            {
              env = {
                QI_MARKET_GO = "${contracts.packages.${pkgs.stdenv.hostPlatform.system}.market-go}";
                QI_MARKET_VECTORS = "${contracts}/market/vectors";
                QI_DEFINITION = "${definition.packages.${pkgs.stdenv.hostPlatform.system}.qi-v2-platform-definition}";
              };
              hook = ''
                r=$(git rev-parse --show-toplevel 2>/dev/null) &&
                  grep -qx 'module github.com/zhifengzhang-sz/qi-v2-platform/core' "$r/core/go.mod" 2>/dev/null &&
                  make -C "$r" -s go.work
              '';
            }
          ];
          packages = with pkgs; [
            gnumake git gh curl python3
            # git's transport to github.com (the remote is git@github.com)
            openssh
          ];
          # of the host, only what nixpkgs cannot give: Nix itself and the client of the machine's Docker engine
          hostTools = [ "nix" "nix-store" "docker" ];
          needs = [
            { tool = "docker"; by = "the services stack and its test stack, qi-v2-services' Redpanda, TimescaleDB and ClickHouse, which make start and the tests that reach a stack need"; }
          ];
          # this project's shell, which the Makefile and the pre-commit hook look for
          env = { PLATFORM_SHELL = "1"; };
          closing = ''echo "platform: make help lists the targets; docs/opening.md says where the work stands"'';
        };
      });
      # The charting consumer's page (docs/designs/design-chart-consumer-2026-11-09.md, Its build), served by bin/chart
      # from bin/chart-page, where make build links it: its main.ts type-checked by TypeScript as @qi/chart's README
      # says a taker's takes the sources (consumers/chart/page/tsconfig.json), against the package qi-chart's
      # node_modules, then bundled by Bun for the browser into chart.js; beside it index.html, the package's tv engine
      # (tv.js), and Lightweight Charts' standalone build and its licence. Bun and TypeScript are this flake's nixpkgs',
      # the nixpkgs visualization builds its engines' scripts with; neither is in the shell. The program's page, no
      # product.
      packages = forAll (pkgs:
        let v = visualization.packages.${pkgs.stdenv.hostPlatform.system};
        in {
          # The install of the K-line data sets, the product datasets, of its number
          # (docs/designs/design-klines-dataset-2026-11-09.md, How a Nix taker takes a version): datasets-install
          # DATASET VERSION, datasets/install.py run by this nixpkgs' python3 (-I), given this commit's locks
          # (datasets/*.lock alone) and crane, nixpkgs'; each file the store lacks fetched from GHCR with no login, and
          # added by the taker's own nix-store once it is the lock's sha256 and size.
          datasets =
            let
              version = "1.0.0";
              locks = builtins.path {
                name = "datasets-locks";
                path = ./datasets;
                filter = p: t: t == "regular" && pkgs.lib.hasSuffix ".lock" p;
              };
            in
            pkgs.runCommand "datasets-${version}" { inherit version; meta.mainProgram = "datasets-install"; } ''
              mkdir -p $out/bin
              cat > $out/bin/datasets-install <<'EOF'
              #!${pkgs.runtimeShell}
              exec ${pkgs.python3}/bin/python3 -I ${./datasets/install.py} ${locks} ${pkgs.crane}/bin/crane "$@"
              EOF
              chmod +x $out/bin/datasets-install
            '';
          chart-page = pkgs.runCommand "chart-page" { nativeBuildInputs = [ pkgs.bun pkgs.typescript ]; } ''
            export HOME=$TMPDIR
            cp ${./consumers/chart/page/main.ts} main.ts
            cp ${./consumers/chart/page/tsconfig.json} tsconfig.json
            ln -s ${v.qi-chart}/node_modules node_modules
            tsc -p tsconfig.json
            mkdir -p $out
            bun build main.ts --target=browser --format=iife --minify --outfile $out/chart.js
            cp ${./consumers/chart/page/index.html} $out/index.html
            cp ${v.qi-chart}/node_modules/@qi/chart/engine/tv.js $out/tv.js
            cp ${v.lightweight-charts}/dist/lightweight-charts.standalone.production.js $out/lightweight-charts.js
            cp ${v.lightweight-charts}/LICENSE $out/LICENSE.lightweight-charts
          '';
        });
      # A data set's version for a taker's package, of the product datasets (the same design): the lock of `dataset` at
      # this commit, its version `version`, else the evaluation stops, named; each file by requireFile in the taker's
      # nixpkgs, its store path the one its name (its path in the lock) and its sha256 fix, whatever that nixpkgs,
      # filled by the install alone; folder, one folder linking every file by its path; the lock's dataset, version,
      # image and digest, which anything made from it names.
      lib.dataset = { pkgs, dataset, version }:
        let
          l = pkgs.lib;
          have = map (l.removeSuffix ".lock") (builtins.filter (l.hasSuffix ".lock") (builtins.attrNames (builtins.readDir ./datasets)));
          lock =
            if builtins.match "[a-z0-9]+([._-][a-z0-9]+)*" dataset == null then
              throw "lib.dataset: \"${dataset}\": not a data set's name (lower-case letters and digits, a '.', '_' or '-' between them)"
            else if !(builtins.elem dataset have) then
              throw "lib.dataset: no data set ${dataset} in the platform at this commit; its data sets: ${l.concatStringsSep ", " have}"
            else
              builtins.fromJSON (builtins.readFile (./datasets + "/${dataset}.lock"));
          files = builtins.listToAttrs (map (f: {
            name = f.path;
            value = pkgs.requireFile {
              name = f.path;
              sha256 = f.sha256;
              message = ''
                ${dataset} ${version}: ${f.path} (sha256 ${f.sha256}) is not in the Nix store. The version is installed
                from GHCR, with no login, by the platform's flake at the commit this flake's lock pins:
                  nix run --inputs-from . platform#datasets -- ${dataset} ${version}
                (platform: this flake's name of the platform's input)
              '';
            };
          }) lock.files);
        in
        if lock.version != version then
          throw "lib.dataset: ${dataset} at this commit is ${lock.version}, not ${version}: a version is taken at a tag of the platform whose lock is that version"
        else {
          inherit (lock) dataset version image digest;
          inherit files;
          folder = pkgs.linkFarm "${dataset}-${version}" (map (f: { name = f.path; path = files.${f.path}; }) lock.files);
        };
    };
}
