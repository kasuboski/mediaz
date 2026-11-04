{ lib, buildGoModule, fetchFromGitHub, go }:

# Build with the same Go toolchain as the development shell.
(buildGoModule.override { inherit go; }) {
  pname = "gremlins";
  version = "0.6.0";

  src = fetchFromGitHub {
    owner = "go-gremlins";
    repo = "gremlins";
    rev = "v0.6.0";
    hash = "sha256-QwMj7aA4eafMT25gBLAomZMliCbueoEsDHD/nxtnmk4=";
  };

  vendorHash = "sha256-TYbbDN2V6GLj+YRNQIKggCnNspk3M96cP1DSe8P9qlY=";
  subPackages = [ "cmd/gremlins" ];
  # Go test exits 1 for build failures too. Use structured build events so an
  # uncompilable mutation cannot masquerade as a successful assertion kill.
  patches = [ ./gremlins-build-failures.patch ];
  # The patch changes subprocess args expected by upstream executor unit tests.
  # The repository's mutate-selftest checks the built binary end to end instead.
  doCheck = false;

  meta = {
    description = "Mutation testing for Go";
    homepage = "https://github.com/go-gremlins/gremlins";
    license = lib.licenses.asl20;
    mainProgram = "gremlins";
  };
}
