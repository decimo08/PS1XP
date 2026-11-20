# PS1XP · PS1 to EBOOT for Windows x64

Convert PS1 BIN, CUE, and ISO disc images to `EBOOT.PBP` for the PSP. PS1XP is an independent Windows interface using [PSXPackager 1.8.0](https://github.com/RupertAvery/PSXPackager/tree/v1.8.0) by **RupertAvery (David Khristepher Santos)**.

## Download and first run

1. Download the [offline portable Windows x64 ZIP](https://github.com/decimo08/PS1XP/releases/tag/v0.1.0-rc.5) and extract it.
2. Run **`PS1 to EBOOT.exe`**. The patched engine, .NET runtime, and matching PSXPackager resources are included. Conversion works offline on first launch.

Drop a disc image or ZIP onto **Convert**, or use **Choose file**. For a multi-track disc, use the CUE and its BIN tracks. A ZIP may contain one CUE with its referenced tracks, or one standalone BIN or ISO. The app stages ZIP files internally. Select a save folder and click **Create EBOOT.PBP**. An export is saved as `<save folder>/<game ID>/EBOOT.PBP`.

## Features

- Game ID detection, live conversion progress, cancellation, and overwrite confirmation.
- Three lossless compression presets and one-disc-per-export support.
- Default artwork, custom PNG or JPEG, online cover lookup, and a PSP-style artwork preview.
- PMF import. Cutscene-to-PMF conversion and playback preview use [optional tools](BUILD.md#optional-pmf-tools).

To restore the photo frame from the private build, use **Artwork → Choose preview frame** and select your original 720 × 504 `PSP_Frame.png`. PS1XP saves a local copy for future launches.

The PSP menu displays PMF animation in its icon slot. The full-screen background is a static PNG. Cooked ISO images can omit original CD audio or CD-XA data.

## Verification and credits

The maintainer verified that the previously failing title boots on a physical PSP using the corrected private Windows build. This public build uses that same loader path. The bundled resources and generated PBP sections were tested in the build environment. The public Windows UI and PSP playback still need an end-to-end test.

The engine resources are copied unchanged from the upstream v1.8.0 distribution. The interface and original artwork are covered by the [MIT license](LICENSE). Upstream notices are in [notices](notices), and the engine patches are in [patches](patches). See [BUILD.md](BUILD.md) for source builds and [RELEASE.md](RELEASE.md) for release notes.
