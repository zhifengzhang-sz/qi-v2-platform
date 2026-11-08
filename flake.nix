# The platform's tools, the same on every machine (docs/process.md): `nix develop`, then `make install` and the
# targets of the Makefile. Everything from outside is an input pinned by flake.lock: qi-v2's one nixpkgs, its pure
# shell and its Go preset; the kit, which writes the agents' files from agents/; the services stack, the package
# `stack` of qi-v2-services, run by make stack-up and make stack-down on the machine's Docker engine, and its test
# stack, by make stack-test-up and make stack-test-down; market's Go form, the package market-go of qi-v2-contracts,
# which the Go workspace (go.work) takes, and market's vectors, which the library's tests read. Every input is a
# public repository, read with no login.
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

  outputs = { self, nixpkgs, qi-v2, kit, services, contracts }:
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
          # lib/massive's tests read
          uses = [
            (qi-v2.lib.go pkgs)
            {
              env = {
                QI_MARKET_GO = "${contracts.packages.${pkgs.stdenv.hostPlatform.system}.market-go}";
                QI_MARKET_VECTORS = "${contracts}/market/vectors";
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
    };
}
