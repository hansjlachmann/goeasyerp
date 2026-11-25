# Security policy

## Supported versions

GoEasyERP is released often, in small steps. Security fixes go into the next release only:
use the latest release (the backend, frontend and builder images on ghcr.io, the compose files and
the install scripts of the same version). Older releases do not get fixes.

| Version              | Fixes        |
| -------------------- | ------------ |
| The latest release   | Yes          |
| Any earlier release  | No — upgrade |

## Reporting a vulnerability

Please do **not** open a public issue, pull request or discussion for a security problem.

Report it privately through GitHub: on this repository's **Security** tab, choose **Report a
vulnerability**. Only the maintainer sees the report.

Please include what you can of:

- what an attacker can do, and what they need first (a user account, a role, access to the
  network or the server);
- the steps to reproduce it, or a proof of concept;
- the version (shown in the app and in the image tag) and how GoEasyERP is installed (one-computer
  trial, server with Docker Compose, Kubernetes);
- whether the problem is already public.

What happens next:

- you get an answer within a week;
- a confirmed problem is fixed in a release as soon as possible, and the release notes say what was
  fixed once installations can upgrade;
- you are credited in the advisory, unless you prefer not to be.

Please test only against an installation of your own (the one-computer trial is quickest: see the
README). Do not test against installations operated by others, and do not read, change or delete
data that is not yours.

## Scope

In scope: the code in this repository and the images built from it — the backend, the frontend,
the app builder, the compose files, the install scripts and the nginx configuration.

Out of scope: how a particular installation is operated (its server, network, passwords and
backups — `docs/operations.md` describes what an operator should do), extension apps from other
repositories, and problems in third-party software that are already fixed in a newer version we
use. Reports of those are still welcome when they affect GoEasyERP's defaults.
