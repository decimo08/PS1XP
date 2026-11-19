# PS1 to EBOOT · Windows x64

A portable Windows x64 interface for creating PSP `EBOOT.PBP` files from PS1 BIN, CUE, and ISO images. Features include drag-and-drop, game ID folders, live progress, icon and background editing, and PMF creation and preview.

The original conversion engine is [PSXPackager 1.8.0](https://github.com/RupertAvery/PSXPackager/tree/v1.8.0) by **RupertAvery (David Khristepher Santos)**. This candidate supplies the Windows interface, original replacement artwork, and PSXPackager patches for console-free progress and the official GUI’s boot resources.

## Downloads

- [Windows x64 launcher ZIP](PS1-to-EBOOT-Launcher-0.1.0-rc.1-win-x64.zip)
- [SHA-256 checksums](SHA256SUMS.txt)
- [Build and setup instructions](BUILD.md)
- [Release notes and Windows smoke test](RELEASE.md)

The launcher ZIP needs the upstream engine and runtime files listed in [BUILD.md](BUILD.md). The optional PMF tools must also be added to enable the cutscene workflow. The download includes original artwork. Obtain the engine resources and optional FFmpeg binaries from their upstream sources as described in the build guide.

## Output and limitations

The app creates `<destination>/<game ID>/EBOOT.PBP`. Use a CUE when the disc includes one and keep all its BIN tracks beside it. The app handles one disc per export. PMF animation appears in the PSP menu icon slot. The full-screen background is a static PNG.

Open-source notices are in [notices](notices/). The interface source and original release artwork are available under the [MIT license](LICENSE). Upstream components retain their own licenses.

## Compatibility

The maintainer confirmed that a game which previously exited on a physical PSP launched successfully after the boot-resource fix in the private Windows build. This source release uses that same patch. The clean launcher package with separately assembled dependencies has not been tested on Windows or PSP hardware.
