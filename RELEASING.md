# Release Process

This document describes the release process for the QQ project.

## Prerequisites

- [goreleaser](https://goreleaser.com/install/) is required to build and publish releases
- GitHub access to create tags and releases
- Docker installed locally for testing container builds

## Release Steps

1. Ensure all changes are committed and pushed to the main branch

2. Run a local test build:
   ```bash
   goreleaser build --clean --snapshot --config .goreleaser-local.yaml
   ```

3. Create and push a new tag:
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

4. The GitHub Actions workflow will automatically:
   - Build binaries for all supported platforms
   - Create packages (deb, rpm, apk)
   - Build and publish Docker images to GitHub Container Registry
   - Create a GitHub release with all artifacts

5. Verify the release on GitHub:
   - Check the GitHub Actions run completed successfully
   - Verify the release page contains all expected assets
   - Verify the Docker image is available on GitHub Container Registry

## Version Scheme

We follow [Semantic Versioning](https://semver.org/):

- MAJOR version for incompatible API changes
- MINOR version for backward-compatible functionality additions
- PATCH version for backward-compatible bug fixes

## Release Checklist

Before creating a new release, check that:

- [ ] All tests pass
- [ ] Documentation is updated
- [ ] CHANGELOG.md is updated
- [ ] Version number is correctly updated in relevant files
- [ ] All new features work as expected

## Post-Release

After a successful release:

1. Update the version in development to the next version with `-dev` suffix
2. Announce the release to the community
3. Verify that the Homebrew formula was created successfully in the homebrew-tap repository
4. Test the Homebrew installation:
   ```bash
   brew update
   brew install willnewby/tap/qq
   qq --version
   ```
5. Update installation instructions if necessary

## Homebrew Tap Setup

To enable Homebrew installations, you need to set up a Homebrew tap repository. See [docs/homebrew-setup.md](docs/homebrew-setup.md) for detailed instructions on setting up your Homebrew tap.