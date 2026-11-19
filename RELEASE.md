# 0.1.0-rc.1 · release notes

This candidate uses original icon, background, and handheld preview artwork. It includes the Windows x64 launcher, buildable interface source, the redirected-console and GUI boot-resource patches, and a SHA-256 checksum.

## Verification

The launcher was cross-compiled and passed Windows-targeted Go vet. The existing conversion, artwork, and sample STR-to-PMF workflow passed automated tests in the build environment. The assembled candidate still needs a Windows smoke test. The maintainer confirmed a successful PSP launch of the previously failing title using the corrected private build. The independently assembled release candidate still needs its own Windows and PSP checks.

## Windows smoke test

- Extract to a fresh writable folder and assemble dependencies per BUILD.md.
- Launch the app. Drop a BIN/CUE with separate track files. Confirm ID detection, live progress, and a valid EBOOT.PBP in the ID folder.
- Try dialog cancellation and an existing output replacement. Verify that cancellation leaves an existing EBOOT.PBP intact.
- Choose a local icon and background. Confirm the preview and embedded artwork.
- Generate a PMF from a supported STR clip and play it in the Artwork preview.
- If a PSP is available, confirm the exported game starts and displays both the static background and animated icon.

## Public release

Publish from a new repository initialized from this source snapshot. Keep the MIT license and third-party notices. Obtain upstream resources separately as documented in BUILD.md. Attach the tested ZIP and SHA-256 file to a versioned release. The release candidate is unsigned. An Authenticode signature from a trusted publisher can help establish trust, and Windows may still warn while a build has limited reputation.
