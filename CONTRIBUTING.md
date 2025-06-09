# Contributing to pulumi-go-provider

## Release Process

To cut and release a new version of this framework:

1. Update the `.version` file at the repository root with the intended next version number without the `v` prefix (e.g. `1.2.3`). Be sure not to add a trailing newline character.
2. Tag the commit with a [semantic version](https://semver.org/) (e.g., `git tag v1.2.3`).
3. Push the tag to GitHub (`git push origin v1.2.3`).

Once the tag is pushed, GitHub Actions CI will automatically handle the rest of the release process, which creates the GitHub release page.
