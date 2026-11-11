# The platform's tools, the same on every machine (docs/process.md): `nix develop`, then `make install` and the
# targets of the Makefile. Everything from outside is an input pinned by flake.lock: qi-v2's one nixpkgs, its pure
# shell and its Go preset; the kit, which writes the agents' files from agents/; the services stack, the package
# `stack` of qi-v2-services, run by make stack-up and make stack-down on the machine's Docker engine, and its test
# stack, by make stack-test-up and make stack-test-down; market's Go form, the package market-go of qi-v2-contracts,
# which the Go workspace (go.work) takes, and market's vectors, which the library's tests read; the platform's formal
# definition, the package qi-v2-platform-definition of the project of that name, its Lean as source, which the shell
# names (QI_DEFINITION). Every input is a public repository, read with no login. The products released as packages
# (PRODUCTS.md): abstract-level, the definition's interfaces in Go (abstract/README.md).
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
  # test stack (make stack-test-up, stack-test-stop, stack-test-down)
  inputs.services.url = "github:zhifengzhang-sz/qi-v2-services?ref=stack-0.3.0";
  inputs.services.inputs.qi-v2.follows = "qi-v2";
  # The market data contract, published by qi-v2-contracts at its tag: its package market-go, market's Go form as
  # source, which replaces the module the library's go.mod requires at v0.0.0 (docs/impl/engineering/go.toolchain.md, 2);
  # its vectors, in the input's source, to which the library holds the base of its K-lines (lib/README.md)
  inputs.contracts.url = "github:zhifengzhang-sz/qi-v2-contracts?ref=market-1.1.0";
  inputs.contracts.inputs.qi-v2.follows = "qi-v2";
  inputs.contracts.inputs.kit.follows = "kit";
  # The platform's formal definition, published by qi-v2-platform-definition at its release's tag: its package
  # qi-v2-platform-definition, the Lake package of its Lean as source (the base, Kafka's topic, the four components and
  # their wiring), which the shell names (QI_DEFINITION). Its qi-v2 and kit follow this flake's, and so its nixpkgs.
  inputs.definition.url = "github:zhifengzhang-sz/qi-v2-platform-definition?ref=qi-v2-platform-definition-0.1.0";
  inputs.definition.inputs.qi-v2.follows = "qi-v2";
  inputs.definition.inputs.kit.follows = "kit";

  outputs = { self, nixpkgs, qi-v2, kit, services, contracts, definition }:
    let
      systems = [ "x86_64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f (import nixpkgs { inherit system; }));
      # Each product's one number (its README, Changes).
      versions = { abstract-level = "0.1.0"; };
    in {
      # abstract-level (abstract/README.md, Taking it): the module of abstract/ as source, without its tests, its files
      # at the package's root, for a taker's go.mod to replace the module's path with; with its README and LICENSE;
      # the standard library alone. Checked as a taker takes it by make check-abstract-package.
      packages = forAll (pkgs: {
        abstract-level = pkgs.runCommand "abstract-level-${versions.abstract-level}" { } ''
          mkdir -p $out
          cp ${./abstract/go.mod} $out/go.mod
          for f in ${./abstract}/*.go; do
            case $f in *_test.go) ;; *) cp $f $out/ ;; esac
          done
          cp ${./abstract/README.md} $out/README.md
          cp ${./LICENSE} $out/LICENSE
        '';
      });


      devShells = forAll (pkgs: {
        default = qi-v2.lib.mkPureShell pkgs {
          name = "platform";
          # Go and gopls, their caches under ~/.cache/platform/; GOTOOLCHAIN=local: the flake's go, never another.
          # Every program is Go. The Go workspace: QI_MARKET_GO, market-go's store path, from which make go.work
          # writes go.work at the root (never committed), written on entry too; QI_MARKET_VECTORS, market's vectors
          # in the input's source (market/vectors), which lib/massive's tests read; QI_DEFINITION, the definition's
          # package, its Lean's files at its root
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
                  grep -qx 'module github.com/zhifengzhang-sz/qi-v2-platform/lib' "$r/lib/go.mod" 2>/dev/null &&
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
            { tool = "docker"; by = "the services stack and its test stack, qi-v2-services' Redpanda, TimescaleDB and ClickHouse, which the stack's targets and the tests that reach a stack need"; }
          ];
          # this project's shell, which the Makefile and the pre-commit hook look for
          env = { PLATFORM_SHELL = "1"; };
          closing = ''echo "platform: make help lists the targets; docs/opening.md says where the work stands"'';
        };
      });
    };
}
