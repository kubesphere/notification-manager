# Releases

This page describes the release process and the currently planned schedule for upcoming releases as well as the respective release shepherd.

## Release schedule

| release series | date  (year-month-day) | release shepherd                    |
|----------------|------------------------|-------------------------------------|
| v1.0.0         | 2021-04-22             | leiwanjun (GitHub: @leiwanjun)      |
| v1.2.0         | 2021-07-15             | leiwanjun (GitHub: @leiwanjun)      |
| v1.3.0         | 2021-08-10             | zhu733756 (GitHub: @zhu733756)      |
| v1.4.0         | 2021-10-14             | leiwanjun (GitHub: @leiwanjun)      |
| v2.0.0         | 2022-04-18             | leiwanjun (GitHub: @leiwanjun)      |
| v2.0.1         | 2022-05-13             | leiwanjun (GitHub: @leiwanjun)      |
| v2.1.0         | 2022-11-29             | leiwanjun (GitHub: @leiwanjun)      |
| v2.2.0         | 2023-01-06             | Gentleelephant (GitHub: @Gentleelephant) |
| v2.3.0-rc.0    | 2023-02-07             | leiwanjun (GitHub: @leiwanjun)      |
| v2.3.0         | 2023-04-12             | leiwanjun (GitHub: @leiwanjun)      |
| v2.4.0         | 2023-09-20             | leiwanjun (GitHub: @leiwanjun)      |
| v2.5.0         | 2024-03-21             | leiwanjun (GitHub: @leiwanjun)      |
| v2.5.1         | 2024-04-03             | leiwanjun (GitHub: @leiwanjun)      |
| v2.5.2         | 2024-04-14             | leiwanjun (GitHub: @leiwanjun)      |
| v2.5.3         | 2025-03-10             | Gentleelephant (GitHub: @Gentleelephant) |


# How to cut a new release

> This guide is strongly based on the [Prometheus release instructions](https://github.com/prometheus/prometheus/blob/master/RELEASE.md).

## Branch management and versioning strategy

We use [Semantic Versioning](http://semver.org/).

We maintain a separate branch for each minor release, named `release-<major>.<minor>`, e.g. `release-1.1`, `release-2.0`.

The usual flow is to merge new features and changes into the main branch and to merge bug fixes into the latest release branch. Bug fixes are then merged into main from the latest release branch. The main branch should always contain all commits from the latest release branch.

If a bug fix got accidentally merged into main, cherry-pick commits have to be created in the latest release branch, which then have to be merged back into main. Try to avoid that situation.

Maintaining the release branches for older minor releases happens on a best effort basis.

## Prepare your release

For a new major or minor release, work from the `main` branch. For a patch release, work in the branch of the minor release you want to patch (e.g. `release-0.1` if you're releasing `v0.1.1`).

<!-- Change the `Install the latest stable version` section in README.md to the new stable version:
```bash
kubectl apply -f https://raw.githubusercontent.com/kubesphere/notification-manager/master/config/bundle.yaml
``` -->

Add an entry for the new version to the `CHANGELOG.md` file. Entries in the `CHANGELOG.md` are meant to be in this order:

* `[CHANGE]`
* `[FEATURE]`
* `[ENHANCEMENT]`
* `[BUGFIX]`

Create a PR for the changes to be reviewed.

## Publish the new release

For new minor and major releases, create the `release-<major>.<minor>` branch starting at the PR merge commit.
From now on, all work happens on the `release-<major>.<minor>` branch.

Bump the version in the `VERSION` file in the root of the repository.

Make sure you've set up docker buildx environment, then run the following command to build and push amd64 and arm64 images at the same time:
make build:

```bash
make build
```
> We'll add a CI pipeline in the future which will automatically push the container images to [docker hub](https://hub.docker.com/repository/docker/openfunction).

Tag the new release with a tag named `v<major>.<minor>.<patch>`, e.g. `v2.1.3`. Note the `v` prefix. You can do the tagging on the commandline:

```bash
tag="$(< VERSION)"
git tag -a "${tag}" -m "${tag}"
git push origin "${tag}"
```
Commit all the changes.

Go to https://github.com/kubesphere/notification-manager/releases, associate the new release with the before pushed tag, paste in changes made to `CHANGELOG.md`, add file `config/bundle.yaml` and then click "Publish release".

For patch releases, submit a pull request to merge back the release branch into the `master` branch.
