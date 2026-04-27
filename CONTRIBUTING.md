# Contributing to pulumi-go-provider

## Release Process

To cut and release a new version of this framework:

1. Tag the commit with a [semantic version](https://semver.org/) (e.g., `git tag v1.2.3`).
2. Push the tag to GitHub (`git push origin v1.2.3`).

Once the tag is pushed, GitHub Actions CI will automatically handle the rest of the release process, which creates the GitHub release page. The framework reports its own version at runtime via Go's `runtime/debug.ReadBuildInfo`, so no in-repo file needs to be updated.
