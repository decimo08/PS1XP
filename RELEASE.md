# PS1XP v0.1.0-rc.2

- The full Windows x64 portable ZIP includes the patched PSXPackager engine and bundled .NET 10 runtime.
- One-time setup imports checksum-verified PSP resources from the official PSXPackager v1.8.0 ZIP. The source ZIP can be downloaded on demand or selected locally.
- The build includes original default artwork and preserves the PSP boot fix that passed the maintainer’s test of the previously failing title on the private build.
- PMF creation and playback preview require the optional media tools documented in BUILD.md.

The public build is unsigned and has not undergone a full Windows and PSP smoke test. The previous launcher-only v0.1.0-rc.1 package needed manual engine assembly. Windows 10 or 11 x64 is required.
