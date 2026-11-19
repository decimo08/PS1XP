# PS1XP · PS1 to EBOOT for Windows x64

Convert PS1 BIN, CUE, and ISO disc images to `EBOOT.PBP` for the PSP. PS1XP is an independent Windows interface using [PSXPackager 1.8.0](https://github.com/RupertAvery/PSXPackager/tree/v1.8.0) by **RupertAvery (David Khristepher Santos)**.

## Download and first run

1. Download the [full portable prerelease ZIP](https://github.com/decimo08/PS1XP/releases/tag/v0.1.0-rc.2) and extract it to a writable folder.
2. Run **`PS1 to EBOOT.exe`**. The conversion engine and .NET runtime are already in the package.
3. At the one-time setup prompt, select **Yes** to download and import the official PSXPackager resources or **No** to choose an [official PSXPackager v1.8.0 ZIP](https://github.com/RupertAvery/PSXPackager/releases/tag/v1.8.0) you already downloaded. Keep the official ZIP intact for import. The app verifies and saves three resources locally, then starts automatically.

Drop a disc image onto **Convert**, select a save folder, and click **Create EBOOT.PBP**. For a multi-track disc, drop the CUE and keep the referenced BIN tracks beside it. An export is saved as `<save folder>/<game ID>/EBOOT.PBP`.

## Features

- Game ID detection, live conversion progress, cancellation, and overwrite confirmation.
- Three lossless compression presets and one-disc-per-export support.
- Default artwork, custom PNG or JPEG, online cover lookup, and a PSP-style artwork preview.
- PMF import. Cutscene-to-PMF conversion and playback preview use [optional tools](BUILD.md#optional-pmf-tools).

The PSP menu displays PMF animation in its icon slot. The full-screen background is a static PNG. Cooked ISO images can omit original CD audio or CD-XA data.

## Verification and credits

The maintainer verified that the previously failing title boots on a physical PSP using the corrected private Windows build. This public build uses that same loader path. The first-run import and generated PBP sections were tested in the build environment. The public Windows UI and PSP playback still need an end-to-end test.

The interface and original artwork are covered by the [MIT license](LICENSE). Upstream notices are in [notices](notices), and the engine patches are in [patches](patches). See [BUILD.md](BUILD.md) for source builds and [RELEASE.md](RELEASE.md) for release notes.
